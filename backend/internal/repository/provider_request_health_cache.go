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
func providerHealthDegradedAvoidKey(session string) string {
	return providerHealthAvoidKey(session) + ":degraded"
}

// Sorted failure evidence, per-request deduplication, and success fences live in
// Redis together. Independent gateway replicas cannot lose increments or let a
// late completion of an older request reset a newer failure streak.
var observeProviderHealthScript = redis.NewScript(`
local now, started, window, avoid = tonumber(ARGV[1]),tonumber(ARGV[2]),tonumber(ARGV[3]),tonumber(ARGV[4])
local cutoff=now-window
local function observeLane(first, threshold, state)
  if state=='-1' then return end
  local failures, success, request, exclusions, account = KEYS[first],KEYS[first+1],KEYS[first+2],KEYS[first+3],KEYS[first+4]
  redis.call('ZREMRANGEBYSCORE',failures,'-inf',cutoff)
  local succeeded=tonumber(redis.call('GET',success) or '0')
  local accepted=0
  if state=='1' then
    if redis.call('GET',request)=='ok' then return end
    redis.call('SET',request,'bad','PX',window)
    accepted=1
    if started>succeeded and started>cutoff then
      redis.call('ZADD',failures,'NX',started,ARGV[7])
    end
  else
    redis.call('SET',request,'ok','PX',window)
    if started>succeeded then
      succeeded=started
      redis.call('SET',success,started,'PX',window)
    end
    redis.call('ZREMRANGEBYSCORE',failures,'-inf',succeeded)
  end
  redis.call('PEXPIRE',failures,window)
  redis.call('ZREMRANGEBYSCORE',exclusions,'-inf',now)
  local count=redis.call('ZCARD',failures)
  if accepted==1 then
    redis.call('ZREMRANGEBYSCORE',account,'-inf',cutoff)
    redis.call('ZADD',account,'NX',now,ARGV[7])
    redis.call('PEXPIRE',account,window)
  end
  if threshold>0 and count>=threshold then
    redis.call('ZADD',exclusions,now+avoid,ARGV[8])
  elseif state=='0' then
    redis.call('ZREM',exclusions,ARGV[8])
  end
  -- One account's short avoidance must not expire another account's longer one.
  redis.call('PEXPIRE',exclusions,math.max(avoid,window,redis.call('PTTL',exclusions)))
  redis.call('ZREMRANGEBYRANK',failures,0,-10001)
end
observeLane(1,tonumber(ARGV[5]),ARGV[6])
observeLane(6,tonumber(ARGV[9]),ARGV[10])
return 1
`)

func (c *gatewayCache) ObserveProviderRequestHealth(ctx context.Context, e service.ProviderRequestHealthEvent) error {
	base := providerHealthAccountKey(e.AccountID, e.Generation)
	scope := base + ":session:" + e.Session
	member, _ := json.Marshal(service.ProviderSessionAvoidance{AccountID: e.AccountID, Generation: e.Generation})
	failed := 0
	degraded := -1
	if e.Failed {
		failed = 1
	} else if e.Policy.DegradedThreshold > 0 || e.Policy.AccountDegradedThreshold > 0 {
		degraded = 0
		if e.Degraded {
			degraded = 1
		}
	}
	return observeProviderHealthScript.Run(ctx, c.rdb, []string{
		scope + ":failures", scope + ":success", scope + ":request:" + providerHealthDigest(e.RequestID), providerHealthAvoidKey(e.Session), base + ":failed-requests",
		scope + ":degraded", scope + ":fast-success", scope + ":slow-request:" + providerHealthDigest(e.RequestID), providerHealthDegradedAvoidKey(e.Session), base + ":degraded-requests",
	}, e.CompletedAt.UnixMilli(), e.StartedAt.UnixMilli(), e.Policy.WindowSeconds*1000, e.Policy.SessionAvoidSeconds*1000, e.Policy.FailureThreshold, failed, e.RequestID, string(member), e.Policy.DegradedThreshold, degraded).Err()
}

func (c *gatewayCache) ProviderRecentFailures(ctx context.Context, id int64, generation string, now time.Time, windowSeconds int) (int, error) {
	count, err := c.rdb.ZCount(ctx, providerHealthAccountKey(id, generation)+":failed-requests", "("+strconv.FormatInt(now.Add(-time.Duration(windowSeconds)*time.Second).UnixMilli(), 10), "+inf").Result()
	return int(count), err
}
func (c *gatewayCache) ProviderRecentDegradations(ctx context.Context, id int64, generation string, now time.Time, windowSeconds int) (int, error) {
	count, err := c.rdb.ZCount(ctx, providerHealthAccountKey(id, generation)+":degraded-requests", "("+strconv.FormatInt(now.Add(-time.Duration(windowSeconds)*time.Second).UnixMilli(), 10), "+inf").Result()
	return int(count), err
}
func (c *gatewayCache) ProviderSessionAvoidances(ctx context.Context, session string, now time.Time) ([]service.ProviderSessionAvoidance, error) {
	pipe := c.rdb.Pipeline()
	rangeBy := &redis.ZRangeBy{Min: "(" + strconv.FormatInt(now.UnixMilli(), 10), Max: "+inf", Count: 100}
	failures := pipe.ZRangeByScore(ctx, providerHealthAvoidKey(session), rangeBy)
	degraded := pipe.ZRangeByScore(ctx, providerHealthDegradedAvoidKey(session), rangeBy)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.ProviderSessionAvoidance, 0, len(failures.Val())+len(degraded.Val()))
	seen := make(map[service.ProviderSessionAvoidance]bool)
	for _, v := range append(failures.Val(), degraded.Val()...) {
		var a service.ProviderSessionAvoidance
		if json.Unmarshal([]byte(v), &a) == nil && a.AccountID > 0 && !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out, nil
}

var _ service.ProviderRequestHealthCache = (*gatewayCache)(nil)
