//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 覆盖 canvas 平台「按模型分别计费」的五级优先级：
//  1. 模型专属按秒  2. 模型专属按次  3. 分组全局按秒  4. 分组全局按次  5. 内置默认
func TestCalculateJimengVideoCost_ModelPricingPriority(t *testing.T) {
	bs := &BillingService{fallbackPrices: make(map[string]*ModelPricing)}

	modelCfg := &ModelPricingConfig{
		Video: map[string]*ModelVideoPricing{
			"veo-3.1":      {PricePerSecond: priceOf(0.10)},
			"kling-3.0":    {PricePerCount: priceOf(0.30)},
			"both-modes":   {PricePerCount: priceOf(0.30), PricePerSecond: priceOf(0.10)},
			"no-price-set": {},
		},
	}

	tests := []struct {
		name        string
		model       string
		count       int
		seconds     int
		groupConfig *JimengVideoPriceConfig
		wantCost    float64
		wantMode    string
	}{
		{
			name:        "L1 模型专属按秒优先于分组全局",
			model:       "veo-3.1",
			count:       1,
			seconds:     8,
			groupConfig: &JimengVideoPriceConfig{PricePerSecond: priceOf(999), PricePerCount: priceOf(999), ModelPricing: modelCfg},
			wantCost:    0.8, // 0.10 × 8s × 1
			wantMode:    string(BillingModeVideoPerSecond),
		},
		{
			name:        "L1 多条视频按秒累计",
			model:       "veo-3.1",
			count:       3,
			seconds:     4,
			groupConfig: &JimengVideoPriceConfig{ModelPricing: modelCfg},
			wantCost:    1.2, // 0.10 × 4s × 3
			wantMode:    string(BillingModeVideoPerSecond),
		},
		{
			name:        "L2 模型只配按次时按次计费",
			model:       "kling-3.0",
			count:       2,
			seconds:     10,
			groupConfig: &JimengVideoPriceConfig{PricePerSecond: priceOf(999), ModelPricing: modelCfg},
			wantCost:    0.6, // 0.30 × 2
			wantMode:    string(BillingModeVideo),
		},
		{
			name:        "L2 模型两种价都配时按秒优先",
			model:       "both-modes",
			count:       1,
			seconds:     5,
			groupConfig: &JimengVideoPriceConfig{ModelPricing: modelCfg},
			wantCost:    0.5, // 按秒 0.10 × 5
			wantMode:    string(BillingModeVideoPerSecond),
		},
		{
			name:        "L2 模型配按秒但时长为 0 时降级为模型按次",
			model:       "both-modes",
			count:       2,
			seconds:     0,
			groupConfig: &JimengVideoPriceConfig{ModelPricing: modelCfg},
			wantCost:    0.6, // 0.30 × 2
			wantMode:    string(BillingModeVideo),
		},
		{
			name:        "L3 模型未配置时回退分组全局按秒",
			model:       "seedance-2.0",
			count:       1,
			seconds:     6,
			groupConfig: &JimengVideoPriceConfig{PricePerSecond: priceOf(0.02), PricePerCount: priceOf(9), ModelPricing: modelCfg},
			wantCost:    0.12, // 0.02 × 6
			wantMode:    string(BillingModeVideoPerSecond),
		},
		{
			name:        "L3 模型条目存在但无价格时回退分组全局按秒",
			model:       "no-price-set",
			count:       1,
			seconds:     6,
			groupConfig: &JimengVideoPriceConfig{PricePerSecond: priceOf(0.02), ModelPricing: modelCfg},
			wantCost:    0.12,
			wantMode:    string(BillingModeVideoPerSecond),
		},
		{
			name:        "L4 分组只配按次",
			model:       "seedance-2.0",
			count:       2,
			seconds:     6,
			groupConfig: &JimengVideoPriceConfig{PricePerCount: priceOf(0.25), ModelPricing: modelCfg},
			wantCost:    0.5,
			wantMode:    string(BillingModeVideo),
		},
		{
			name:        "L5 完全无配置走内置默认",
			model:       "seedance-2.0",
			count:       2,
			seconds:     6,
			groupConfig: nil,
			wantCost:    DefaultVideoPricePerCount * 2,
			wantMode:    string(BillingModeVideo),
		},
		{
			name:        "L5 ModelPricing 为 nil 时不影响回退",
			model:       "veo-3.1",
			count:       1,
			seconds:     8,
			groupConfig: &JimengVideoPriceConfig{},
			wantCost:    DefaultVideoPricePerCount,
			wantMode:    string(BillingModeVideo),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bs.CalculateJimengVideoCost(tc.model, tc.count, tc.seconds, tc.groupConfig, 1.0)
			require.NotNil(t, got)
			require.InDelta(t, tc.wantCost, got.TotalCost, 1e-9)
			require.InDelta(t, tc.wantCost, got.ActualCost, 1e-9)
			require.Equal(t, tc.wantMode, got.BillingMode)
		})
	}
}

func TestCalculateJimengVideoCost_RateMultiplier(t *testing.T) {
	bs := &BillingService{fallbackPrices: make(map[string]*ModelPricing)}
	cfg := &JimengVideoPriceConfig{
		ModelPricing: &ModelPricingConfig{
			Video: map[string]*ModelVideoPricing{"veo-3.1": {PricePerSecond: priceOf(0.10)}},
		},
	}

	t.Run("倍率作用于 ActualCost 而非 TotalCost", func(t *testing.T) {
		got := bs.CalculateJimengVideoCost("veo-3.1", 1, 8, cfg, 1.5)
		require.InDelta(t, 0.8, got.TotalCost, 1e-9)
		require.InDelta(t, 1.2, got.ActualCost, 1e-9)
	})

	t.Run("负倍率被夹到 0", func(t *testing.T) {
		got := bs.CalculateJimengVideoCost("veo-3.1", 1, 8, cfg, -1)
		require.InDelta(t, 0.8, got.TotalCost, 1e-9)
		require.Zero(t, got.ActualCost)
	})

	t.Run("count<=0 返回零费用", func(t *testing.T) {
		got := bs.CalculateJimengVideoCost("veo-3.1", 0, 8, cfg, 1.0)
		require.Zero(t, got.TotalCost)
		require.Zero(t, got.ActualCost)
	})
}

// 图片侧：每个模型按尺寸档位独立定价，优先于分组全局价。
func TestCalculateImageCost_ModelPricingPriority(t *testing.T) {
	bs := &BillingService{fallbackPrices: make(map[string]*ModelPricing)}

	modelCfg := &ModelPricingConfig{
		Image: map[string]*ModelImagePricing{
			"gpt-image-2":   {Price1K: priceOf(0.02), Price2K: priceOf(0.06), Price4K: priceOf(0.18)},
			"nano-banana-2": {Price2K: priceOf(0.008)}, // 只配 2K
		},
	}

	tests := []struct {
		name        string
		model       string
		size        string
		count       int
		groupConfig *ImagePriceConfig
		wantUnit    float64
	}{
		{
			name:        "模型专属 1K 优先于分组全局",
			model:       "gpt-image-2",
			size:        "1K",
			count:       1,
			groupConfig: &ImagePriceConfig{Price1K: priceOf(9), Price2K: priceOf(9), Price4K: priceOf(9), ModelPricing: modelCfg},
			wantUnit:    0.02,
		},
		{
			name:        "模型专属 2K",
			model:       "gpt-image-2",
			size:        "2K",
			count:       2,
			groupConfig: &ImagePriceConfig{ModelPricing: modelCfg},
			wantUnit:    0.06,
		},
		{
			name:        "模型专属 4K",
			model:       "gpt-image-2",
			size:        "4K",
			count:       1,
			groupConfig: &ImagePriceConfig{ModelPricing: modelCfg},
			wantUnit:    0.18,
		},
		{
			name:        "小写尺寸也能命中模型专属价",
			model:       "gpt-image-2",
			size:        "1k",
			count:       1,
			groupConfig: &ImagePriceConfig{ModelPricing: modelCfg},
			wantUnit:    0.02,
		},
		{
			name:        "不同模型同尺寸价格互不影响",
			model:       "nano-banana-2",
			size:        "2K",
			count:       1,
			groupConfig: &ImagePriceConfig{ModelPricing: modelCfg},
			wantUnit:    0.008,
		},
		{
			name:        "模型该档位未配置时回退分组全局价",
			model:       "nano-banana-2",
			size:        "4K",
			count:       1,
			groupConfig: &ImagePriceConfig{Price4K: priceOf(0.05), ModelPricing: modelCfg},
			wantUnit:    0.05,
		},
		{
			name:        "模型完全未配置时回退分组全局价",
			model:       "seedream-5.0-pro",
			size:        "2K",
			count:       3,
			groupConfig: &ImagePriceConfig{Price2K: priceOf(0.01), ModelPricing: modelCfg},
			wantUnit:    0.01,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bs.CalculateImageCost(tc.model, tc.size, tc.count, tc.groupConfig, 1.0)
			require.NotNil(t, got)
			require.InDelta(t, tc.wantUnit*float64(tc.count), got.TotalCost, 1e-9)
		})
	}
}
