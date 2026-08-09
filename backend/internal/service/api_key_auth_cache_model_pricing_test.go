//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 快照会序列化进 Redis。ModelPricing 若在往返中丢失，
// 缓存命中的 canvas 请求会静默按分组全局价计费。
func TestAPIKeyAuthGroupSnapshot_ModelPricingRoundTrip(t *testing.T) {
	snapshot := &APIKeyAuthGroupSnapshot{
		ID:       7,
		Name:     "canvas-group",
		Platform: PlatformCanvas,
		Status:   StatusActive,
		ModelPricing: &ModelPricingConfig{
			Video: map[string]*ModelVideoPricing{
				"veo-3.1":   {PricePerSecond: priceOf(0.02)},
				"kling-3.0": {PricePerCount: priceOf(0.5)},
			},
			Image: map[string]*ModelImagePricing{
				"gpt-image-2": {Price1K: priceOf(0.01), Price2K: priceOf(0.03), Price4K: priceOf(0.09)},
			},
		},
	}

	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"model_pricing"`, "快照 JSON 未包含 model_pricing")

	var decoded APIKeyAuthGroupSnapshot
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.NotNil(t, decoded.ModelPricing)

	videoPrice := decoded.ModelPricing.GetModelVideoPrice("veo-3.1")
	require.NotNil(t, videoPrice)
	require.NotNil(t, videoPrice.PricePerSecond)
	require.InDelta(t, 0.02, *videoPrice.PricePerSecond, 1e-9)

	countPrice := decoded.ModelPricing.GetModelVideoPrice("kling-3.0")
	require.NotNil(t, countPrice)
	require.InDelta(t, 0.5, *countPrice.PricePerCount, 1e-9)

	imagePrice := decoded.ModelPricing.GetModelImagePrice("gpt-image-2", "2K")
	require.NotNil(t, imagePrice)
	require.InDelta(t, 0.03, *imagePrice, 1e-9)
}

func TestAPIKeyAuthGroupSnapshot_NilModelPricingOmitted(t *testing.T) {
	snapshot := &APIKeyAuthGroupSnapshot{ID: 1, Name: "anthropic-group", Platform: PlatformAnthropic}

	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), `"model_pricing"`, "未配置定价时不应写入该字段")

	var decoded APIKeyAuthGroupSnapshot
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Nil(t, decoded.ModelPricing)
}

// 旧版本快照必须被拒绝，否则升级后仍会命中不含 model_pricing 的缓存。
func TestAPIKeyService_RejectsSnapshotWithoutModelPricingSupport(t *testing.T) {
	require.Equal(t, 18, apiKeyAuthSnapshotVersion,
		"新增快照字段后必须递增版本号，否则旧缓存不会失效")

	svc := &APIKeyService{}
	groupID := int64(7)

	_, ok, err := svc.applyAuthCacheEntry("k-legacy-model-pricing", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{
			Version:  apiKeyAuthSnapshotVersion - 1,
			APIKeyID: 1,
			UserID:   2,
			GroupID:  &groupID,
			Status:   StatusActive,
		},
	})
	require.NoError(t, err)
	require.False(t, ok, "低版本快照应被拒绝并回源")
}
