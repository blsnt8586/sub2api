package service

// ModelPricingConfig 分组级别的 Canvas 模型定价配置（存储为 groups.canvas_model_pricing JSONB）。
// 优先级：模型专属定价 > 分组全局定价（video_price_per_count 等） > 系统默认定价。
type ModelPricingConfig struct {
	Video map[string]*ModelVideoPricing `json:"video,omitempty"`
	Image map[string]*ModelImagePricing `json:"image,omitempty"`
}

// ModelVideoPricing 单个视频模型的定价配置。
type ModelVideoPricing struct {
	// PricePerCount 按次计费单价（USD/次），nil 表示不使用此模式
	PricePerCount *float64 `json:"per_count,omitempty"`
	// PricePerSecond 按秒计费单价（USD/秒），非 nil 时优先于 PricePerCount
	PricePerSecond *float64 `json:"per_second,omitempty"`
}

// ModelImagePricing 单个图片模型的定价配置（按尺寸档位）。
type ModelImagePricing struct {
	Price1K *float64 `json:"1k,omitempty"` // USD/张
	Price2K *float64 `json:"2k,omitempty"` // USD/张
	Price4K *float64 `json:"4k,omitempty"` // USD/张
}

// Clone 深拷贝定价配置。map 与内部指针都要复制，
// 否则「复制分组」出来的新分组会和源分组共享同一份定价，改一边影响另一边。
func (cfg *ModelPricingConfig) Clone() *ModelPricingConfig {
	if cfg == nil {
		return nil
	}
	out := &ModelPricingConfig{}
	if cfg.Video != nil {
		out.Video = make(map[string]*ModelVideoPricing, len(cfg.Video))
		for model, price := range cfg.Video {
			if price == nil {
				continue
			}
			out.Video[model] = &ModelVideoPricing{
				PricePerCount:  clonePricePointer(price.PricePerCount),
				PricePerSecond: clonePricePointer(price.PricePerSecond),
			}
		}
	}
	if cfg.Image != nil {
		out.Image = make(map[string]*ModelImagePricing, len(cfg.Image))
		for model, price := range cfg.Image {
			if price == nil {
				continue
			}
			out.Image[model] = &ModelImagePricing{
				Price1K: clonePricePointer(price.Price1K),
				Price2K: clonePricePointer(price.Price2K),
				Price4K: clonePricePointer(price.Price4K),
			}
		}
	}
	return out
}

func clonePricePointer(v *float64) *float64 {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}

// GetModelVideoPrice 从模型定价配置中获取指定模型的视频定价。
// 返回 nil 表示该模型未配置专属定价，应回退到分组全局定价。
func (cfg *ModelPricingConfig) GetModelVideoPrice(model string) *ModelVideoPricing {
	if cfg == nil || len(cfg.Video) == 0 || model == "" {
		return nil
	}
	return cfg.Video[model]
}

// GetModelImagePrice 从模型定价配置中获取指定模型+尺寸的图片定价。
// 返回 nil 表示未配置，应回退到分组全局定价。
func (cfg *ModelPricingConfig) GetModelImagePrice(model string, imageSize string) *float64 {
	if cfg == nil || len(cfg.Image) == 0 || model == "" {
		return nil
	}
	modelPricing := cfg.Image[model]
	if modelPricing == nil {
		return nil
	}
	// 尺寸归一化后再查找
	normalizedSize := NormalizeImageBillingTierOrDefault(imageSize)
	switch normalizedSize {
	case ImageBillingSize1K:
		return modelPricing.Price1K
	case ImageBillingSize2K:
		return modelPricing.Price2K
	case ImageBillingSize4K:
		return modelPricing.Price4K
	default:
		return nil
	}
}
