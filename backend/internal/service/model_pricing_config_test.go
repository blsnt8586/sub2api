//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func priceOf(v float64) *float64 { return &v }

func TestModelPricingConfig_GetModelVideoPrice(t *testing.T) {
	cfg := &ModelPricingConfig{
		Video: map[string]*ModelVideoPricing{
			"veo-3.1":    {PricePerSecond: priceOf(0.02)},
			"kling-3.0":  {PricePerCount: priceOf(0.5)},
			"empty-conf": {},
		},
	}

	t.Run("命中按秒模型", func(t *testing.T) {
		got := cfg.GetModelVideoPrice("veo-3.1")
		require.NotNil(t, got)
		require.NotNil(t, got.PricePerSecond)
		require.InDelta(t, 0.02, *got.PricePerSecond, 1e-9)
		require.Nil(t, got.PricePerCount)
	})

	t.Run("命中按次模型", func(t *testing.T) {
		got := cfg.GetModelVideoPrice("kling-3.0")
		require.NotNil(t, got)
		require.NotNil(t, got.PricePerCount)
		require.InDelta(t, 0.5, *got.PricePerCount, 1e-9)
	})

	t.Run("配置存在但价格为空仍返回非 nil 由计费侧回退", func(t *testing.T) {
		got := cfg.GetModelVideoPrice("empty-conf")
		require.NotNil(t, got)
		require.Nil(t, got.PricePerSecond)
		require.Nil(t, got.PricePerCount)
	})

	t.Run("未配置模型返回 nil", func(t *testing.T) {
		require.Nil(t, cfg.GetModelVideoPrice("seedance-2.0"))
	})

	t.Run("空模型名返回 nil", func(t *testing.T) {
		require.Nil(t, cfg.GetModelVideoPrice(""))
	})

	t.Run("nil 配置安全", func(t *testing.T) {
		var nilCfg *ModelPricingConfig
		require.Nil(t, nilCfg.GetModelVideoPrice("veo-3.1"))
	})

	t.Run("空 Video map 返回 nil", func(t *testing.T) {
		require.Nil(t, (&ModelPricingConfig{}).GetModelVideoPrice("veo-3.1"))
	})
}

func TestModelPricingConfig_GetModelImagePrice(t *testing.T) {
	cfg := &ModelPricingConfig{
		Image: map[string]*ModelImagePricing{
			"gpt-image-2": {
				Price1K: priceOf(0.01),
				Price2K: priceOf(0.03),
				Price4K: priceOf(0.09),
			},
			"nano-banana-2": {Price1K: priceOf(0.005)},
		},
	}

	t.Run("按尺寸档位取价", func(t *testing.T) {
		for _, tc := range []struct {
			size string
			want float64
		}{
			{"1k", 0.01},
			{"2k", 0.03},
			{"4k", 0.09},
		} {
			got := cfg.GetModelImagePrice("gpt-image-2", tc.size)
			require.NotNil(t, got, "size=%s", tc.size)
			require.InDelta(t, tc.want, *got, 1e-9, "size=%s", tc.size)
		}
	})

	t.Run("模型只配了 1K 时其他档位返回 nil", func(t *testing.T) {
		require.NotNil(t, cfg.GetModelImagePrice("nano-banana-2", "1k"))
		require.Nil(t, cfg.GetModelImagePrice("nano-banana-2", "4k"))
	})

	t.Run("未配置模型返回 nil", func(t *testing.T) {
		require.Nil(t, cfg.GetModelImagePrice("seedream-5.0-pro", "1k"))
	})

	t.Run("nil 配置与空 map 安全", func(t *testing.T) {
		var nilCfg *ModelPricingConfig
		require.Nil(t, nilCfg.GetModelImagePrice("gpt-image-2", "1k"))
		require.Nil(t, (&ModelPricingConfig{}).GetModelImagePrice("gpt-image-2", "1k"))
	})
}

// Clone 必须深拷贝：复制分组后改新分组的价格不能影响源分组。
func TestModelPricingConfig_Clone_IsDeep(t *testing.T) {
	src := &ModelPricingConfig{
		Video: map[string]*ModelVideoPricing{
			"veo-3.1": {PricePerSecond: priceOf(0.02), PricePerCount: priceOf(0.4)},
		},
		Image: map[string]*ModelImagePricing{
			"gpt-image-2": {Price1K: priceOf(0.01), Price2K: priceOf(0.03), Price4K: priceOf(0.09)},
		},
	}

	clone := src.Clone()
	require.NotNil(t, clone)

	// 改 clone 的 map 成员
	clone.Video["veo-3.1"].PricePerSecond = priceOf(9.99)
	clone.Image["gpt-image-2"].Price1K = priceOf(9.99)
	clone.Video["new-model"] = &ModelVideoPricing{PricePerCount: priceOf(1)}

	require.InDelta(t, 0.02, *src.Video["veo-3.1"].PricePerSecond, 1e-9, "源分组按秒价被污染")
	require.InDelta(t, 0.01, *src.Image["gpt-image-2"].Price1K, 1e-9, "源分组图片价被污染")
	require.NotContains(t, src.Video, "new-model", "源分组 map 被污染")

	// 指针必须是新对象
	require.NotSame(t, src.Video["veo-3.1"], clone.Video["veo-3.1"])
	require.NotSame(t, src.Image["gpt-image-2"], clone.Image["gpt-image-2"])
}

func TestModelPricingConfig_Clone_EdgeCases(t *testing.T) {
	t.Run("nil 返回 nil", func(t *testing.T) {
		var nilCfg *ModelPricingConfig
		require.Nil(t, nilCfg.Clone())
	})

	t.Run("空配置返回非 nil 空结构", func(t *testing.T) {
		clone := (&ModelPricingConfig{}).Clone()
		require.NotNil(t, clone)
		require.Empty(t, clone.Video)
		require.Empty(t, clone.Image)
	})

	t.Run("map 内 nil 成员被跳过", func(t *testing.T) {
		src := &ModelPricingConfig{
			Video: map[string]*ModelVideoPricing{"bad": nil, "ok": {PricePerCount: priceOf(1)}},
			Image: map[string]*ModelImagePricing{"bad": nil},
		}
		clone := src.Clone()
		require.NotContains(t, clone.Video, "bad")
		require.Contains(t, clone.Video, "ok")
		require.NotContains(t, clone.Image, "bad")
	})

	t.Run("nil 价格指针保持 nil", func(t *testing.T) {
		src := &ModelPricingConfig{
			Video: map[string]*ModelVideoPricing{"m": {PricePerSecond: priceOf(0.1)}},
		}
		clone := src.Clone()
		require.Nil(t, clone.Video["m"].PricePerCount)
		require.NotNil(t, clone.Video["m"].PricePerSecond)
	})
}
