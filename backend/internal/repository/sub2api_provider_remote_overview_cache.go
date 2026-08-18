package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	providerRemoteOverviewKeyPrefix = "sub2api:provider:remote-overview:"
	providerRemoteOverviewTTL       = 24 * time.Hour
)

var storeProviderRemoteOverviewSuccessScript = redis.NewScript(`
local current_snapshot = tonumber(redis.call('HGET', KEYS[1], 'snapshot_at_ms') or '0')
local incoming_snapshot = tonumber(ARGV[1])
if incoming_snapshot >= current_snapshot then
  redis.call('HSET', KEYS[1], 'snapshot_at_ms', ARGV[1], 'snapshot_json', ARGV[2])
end
local current_attempt = tonumber(redis.call('HGET', KEYS[1], 'attempted_at_ms') or '0')
local incoming_attempt = tonumber(ARGV[3])
if incoming_attempt >= current_attempt then
  redis.call('HSET', KEYS[1], 'attempted_at_ms', ARGV[3], 'attempt_source', ARGV[4], 'last_error', '', 'last_error_at_ms', '0')
end
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1
`)

var storeProviderRemoteOverviewFailureScript = redis.NewScript(`
local current_attempt = tonumber(redis.call('HGET', KEYS[1], 'attempted_at_ms') or '0')
local incoming_attempt = tonumber(ARGV[1])
if incoming_attempt >= current_attempt then
  redis.call('HSET', KEYS[1], 'attempted_at_ms', ARGV[1], 'attempt_source', ARGV[2], 'last_error', ARGV[3], 'last_error_at_ms', ARGV[1])
end
redis.call('PEXPIRE', KEYS[1], ARGV[4])
return 1
`)

type sub2APIProviderRemoteOverviewCache struct {
	rdb *redis.Client
}

func NewSub2APIProviderRemoteOverviewCache(rdb *redis.Client) service.Sub2APIProviderRemoteOverviewCache {
	return &sub2APIProviderRemoteOverviewCache{rdb: rdb}
}

func providerRemoteOverviewKey(providerID int64) string {
	return fmt.Sprintf("%s%d", providerRemoteOverviewKeyPrefix, providerID)
}

func (c *sub2APIProviderRemoteOverviewCache) GetMany(ctx context.Context, providerIDs []int64) (map[int64]*service.Sub2APIProviderRemoteOverview, error) {
	result := make(map[int64]*service.Sub2APIProviderRemoteOverview, len(providerIDs))
	if len(providerIDs) == 0 {
		return result, nil
	}
	pipe := c.rdb.Pipeline()
	commands := make([]*redis.MapStringStringCmd, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		commands = append(commands, pipe.HGetAll(ctx, providerRemoteOverviewKey(providerID)))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	for index := range commands {
		fields, err := commands[index].Result()
		if err != nil || len(fields) == 0 {
			continue
		}
		overview, err := decodeProviderRemoteOverview(providerIDs[index], fields)
		if err != nil {
			return nil, err
		}
		result[providerIDs[index]] = overview
	}
	return result, nil
}

func decodeProviderRemoteOverview(providerID int64, fields map[string]string) (*service.Sub2APIProviderRemoteOverview, error) {
	overview := &service.Sub2APIProviderRemoteOverview{ProviderID: providerID, Groups: []service.Sub2APIProviderRemoteGroupRate{}}
	if payload := fields["snapshot_json"]; payload != "" {
		if err := json.Unmarshal([]byte(payload), overview); err != nil {
			return nil, fmt.Errorf("decode provider %d remote overview: %w", providerID, err)
		}
	}
	overview.ProviderID = providerID
	if overview.Groups == nil {
		overview.Groups = []service.Sub2APIProviderRemoteGroupRate{}
	}
	if attemptedAtMS, _ := strconv.ParseInt(fields["attempted_at_ms"], 10, 64); attemptedAtMS > 0 {
		overview.LastAttemptedAt = time.UnixMilli(attemptedAtMS).UTC()
	}
	overview.LastAttemptSource = fields["attempt_source"]
	if message := fields["last_error"]; message != "" {
		overview.LastError = &message
		if errorAtMS, _ := strconv.ParseInt(fields["last_error_at_ms"], 10, 64); errorAtMS > 0 {
			errorAt := time.UnixMilli(errorAtMS).UTC()
			overview.LastErrorAt = &errorAt
		}
	}
	return overview, nil
}

func (c *sub2APIProviderRemoteOverviewCache) StoreSuccess(ctx context.Context, overview *service.Sub2APIProviderRemoteOverview) error {
	if overview == nil || overview.ProviderID <= 0 {
		return nil
	}
	payload, err := json.Marshal(overview)
	if err != nil {
		return fmt.Errorf("encode provider remote overview: %w", err)
	}
	return storeProviderRemoteOverviewSuccessScript.Run(
		ctx,
		c.rdb,
		[]string{providerRemoteOverviewKey(overview.ProviderID)},
		overview.SampledAt.UnixMilli(),
		payload,
		overview.LastAttemptedAt.UnixMilli(),
		overview.LastAttemptSource,
		providerRemoteOverviewTTL.Milliseconds(),
	).Err()
}

func (c *sub2APIProviderRemoteOverviewCache) StoreFailure(ctx context.Context, providerID int64, source string, attemptedAt time.Time, errorMessage string) error {
	return storeProviderRemoteOverviewFailureScript.Run(
		ctx,
		c.rdb,
		[]string{providerRemoteOverviewKey(providerID)},
		attemptedAt.UnixMilli(),
		source,
		errorMessage,
		providerRemoteOverviewTTL.Milliseconds(),
	).Err()
}
