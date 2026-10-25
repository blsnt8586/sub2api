package service

// imageCountBillingEnabled 表示该分组强制按实际出图张数计费，即使渠道是 token 计费。
func imageCountBillingEnabled(apiKey *APIKey) bool {
	return apiKey != nil && apiKey.Group != nil && apiKey.Group.ImageCountBilling
}

// imageCountBillingApplies 判断分组按张计费开关是否作用于本次请求。
// 开关只影响专用图片端点；混合的 Responses/Chat/WS 请求即使带图片工具，
// 也必须保留 token 计费，避免整条请求被按图片单价结算而丢失文本 token 费用。
func imageCountBillingApplies(apiKey *APIKey, imageEndpoint bool) bool {
	return imageEndpoint && imageCountBillingEnabled(apiKey)
}

func imagePriceConfigFromAPIKey(apiKey *APIKey) *ImagePriceConfig {
	if apiKey == nil || apiKey.Group == nil {
		return nil
	}
	return &ImagePriceConfig{
		Price1K:      apiKey.Group.ImagePrice1K,
		Price2K:      apiKey.Group.ImagePrice2K,
		Price4K:      apiKey.Group.ImagePrice4K,
		ModelPricing: apiKey.Group.CanvasModelPricing,
	}
}

// canvasImagePriceConfigFromAPIKey preserves Canvas pricing precedence:
// per-model tier price > Canvas global per-count price > generic image tier
// price > the BillingService default. Reusing one price pointer for all tiers
// is intentional because canvas_image_price_per_count has no size dimension.
func canvasImagePriceConfigFromAPIKey(apiKey *APIKey) *ImagePriceConfig {
	config := imagePriceConfigFromAPIKey(apiKey)
	if apiKey == nil || apiKey.Group == nil {
		return config
	}
	if config == nil {
		config = &ImagePriceConfig{}
	}
	if price := apiKey.Group.CanvasImagePricePerCount; price != nil {
		config.Price1K = price
		config.Price2K = price
		config.Price4K = price
	}
	return config
}

func apiKeyHasConfiguredImagePrice(apiKey *APIKey, imageSize string) bool {
	if apiKey == nil || apiKey.Group == nil {
		return false
	}
	if apiKey.Group.GetImagePrice(imageSize) != nil {
		return true
	}
	// 分组配置了 per-model 图片定价时也算"已配置"，确保不会跳到渠道定价路径
	// 从而绕过 ModelPricing（渠道定价优先级应低于显式 per-model 配置）。
	return apiKey.Group.CanvasModelPricing != nil && len(apiKey.Group.CanvasModelPricing.Image) > 0
}

func videoPriceConfigFromAPIKey(apiKey *APIKey) *VideoPriceConfig {
	if apiKey == nil || apiKey.Group == nil {
		return nil
	}
	return &VideoPriceConfig{
		Price480P:   apiKey.Group.VideoPrice480P,
		Price720P:   apiKey.Group.VideoPrice720P,
		Price1080P:  apiKey.Group.VideoPrice1080P,
		ModelPrices: apiKey.Group.VideoModelPrices,
	}
}

func apiKeyHasConfiguredVideoPrice(apiKey *APIKey, model, resolution string) bool {
	return apiKey != nil && apiKey.Group != nil && apiKey.Group.GetVideoPriceForModel(model, resolution) != nil
}

func webSearchPricePerCallFromAPIKey(apiKey *APIKey) *float64 {
	if apiKey == nil || apiKey.Group == nil {
		return nil
	}
	return apiKey.Group.WebSearchPricePerCall
}

func groupSearchPricePer1kFromAPIKey(apiKey *APIKey) *float64 {
	if apiKey == nil || apiKey.Group == nil {
		return nil
	}
	return apiKey.Group.GetSearchPricePer1k()
}

func groupAudioPriceConfigFromAPIKey(apiKey *APIKey) *audioPriceConfig {
	if apiKey == nil || apiKey.Group == nil {
		return nil
	}
	g := apiKey.Group
	return &audioPriceConfig{
		RealtimePerMin: g.AudioRealtimePricePerMin,
		TTSPerMChars:   g.AudioTTSPricePerMillionChars,
		STTPerHour:     g.AudioSTTPricePerHour,
	}
}
