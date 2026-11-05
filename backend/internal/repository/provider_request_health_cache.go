package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

func providerHealthDigest(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
func providerHealthAccountKey(id int64, generation string) string {
	return fmt.Sprintf("provider_health:v1:account:%d:%s", id, providerHealthDigest(generation))
}
func providerHealthAvoidKey(session string) string { return "provider_health:v1:avoid:" + session }

// Sorted failure evidence, per-request deduplication, and success fences live in
// Redis together. Independent gateway replicas cannot lose increments or let a
// late completion of an older request reset a newer failure streak.
var observeProviderHealthScript = redis.NewScript(`
local now, started, window, avoid, threshold = tonumber(ARGV[1]),tonumber(ARGV[2]),tonumber(ARGV[3]),tonumber(ARGV[4]),tonumber(ARGV[5])
local cutoff=now-window
redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',cutoff)
local succeeded=tonumber(redis.call('GET',KEYS[2]) or '0')
local accepted=0
if ARGV[6]=='1' then
  if redis.call('GET',KEYS[3])=='ok' then return 0 end
  redis.call('SET',KEYS[3],'bad','PX',window)
  accepted=1
  if started>succeeded and started>cutoff then
    redis.call('ZADD',KEYS[1],'NX',started,ARGV[7])
  end
else
  redis.call('SET',KEYS[3],'ok','PX',window)
  if started>succeeded then
    succeeded=started
    redis.call('SET',KEYS[2],started,'PX',window)
  end
  redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',succeeded)
end
redis.call('PEXPIRE',KEYS[1],window)
redis.call('ZREMRANGEBYSCORE',KEYS[4],'-inf',now)
local count=redis.call('ZCARD',KEYS[1])
if accepted==1 then
  redis.call('ZREMRANGEBYSCORE',KEYS[5],'-inf',cutoff)
  redis.call('ZADD',KEYS[5],'NX',now,ARGV[7])
  redis.call('PEXPIRE',KEYS[5],window)
end
if count>=threshold then
  redis.call('ZADD',KEYS[4],now+avoid,ARGV[8])
elseif ARGV[6]=='0' then
  redis.call('ZREM',KEYS[4],ARGV[8])
end
redis.call('PEXPIRE',KEYS[4],math.max(avoid,window))
-- Bound per-session data under extreme client concurrency.
redis.call('ZREMRANGEBYRANK',KEYS[1],0,-10001)
return count
`)

func (c *gatewayCache) ObserveProviderRequestHealth(ctx context.Context, e service.ProviderRequestHealthEvent) error {
	base := providerHealthAccountKey(e.AccountID, e.Generation)
	scope := base + ":session:" + e.Session
	member, _ := json.Marshal(service.ProviderSessionAvoidance{AccountID: e.AccountID, Generation: e.Generation})
	failed := 0
	if e.Failed {
		failed = 1
	}
	return observeProviderHealthScript.Run(ctx, c.rdb, []string{scope + ":failures", scope + ":success", scope + ":request:" + providerHealthDigest(e.RequestID), providerHealthAvoidKey(e.Session), base + ":failed-requests"},
		e.CompletedAt.UnixMilli(), e.StartedAt.UnixMilli(), e.Policy.WindowSeconds*1000, e.Policy.SessionAvoidSeconds*1000, e.Policy.FailureThreshold, failed, e.RequestID, string(member)).Err()
}

func (c *gatewayCache) ProviderRecentFailures(ctx context.Context, id int64, generation string, now time.Time, windowSeconds int) (int, error) {
	count, err := c.rdb.ZCount(ctx, providerHealthAccountKey(id, generation)+":failed-requests", "("+strconv.FormatInt(now.Add(-time.Duration(windowSeconds)*time.Second).UnixMilli(), 10), "+inf").Result()
	return int(count), err
}
func (c *gatewayCache) ProviderSessionAvoidances(ctx context.Context, session string, now time.Time) ([]service.ProviderSessionAvoidance, error) {
	values, err := c.rdb.ZRangeByScore(ctx, providerHealthAvoidKey(session), &redis.ZRangeBy{Min: "(" + strconv.FormatInt(now.UnixMilli(), 10), Max: "+inf", Count: 100}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]service.ProviderSessionAvoidance, 0, len(values))
	for _, v := range values {
		var a service.ProviderSessionAvoidance
		if json.Unmarshal([]byte(v), &a) == nil && a.AccountID > 0 {
			out = append(out, a)
		}
	}
	return out, nil
}

var _ service.ProviderRequestHealthCache = (*gatewayCache)(nil)
