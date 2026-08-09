package service

func imagePriceConfigFromAPIKey(apiKey *APIKey) *ImagePriceConfig {
	if apiKey == nil || apiKey.Group == nil {
		return nil
	}
	return &ImagePriceConfig{
		Price1K:      apiKey.Group.ImagePrice1K,
		Price2K:      apiKey.Group.ImagePrice2K,
		Price4K:      apiKey.Group.ImagePrice4K,
		ModelPricing: apiKey.Group.ModelPricing,
	}
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
	return apiKey.Group.ModelPricing != nil && len(apiKey.Group.ModelPricing.Image) > 0
}

func videoPriceConfigFromAPIKey(apiKey *APIKey) *VideoPriceConfig {
	if apiKey == nil || apiKey.Group == nil {
		return nil
	}
	return &VideoPriceConfig{
		Price480P:  apiKey.Group.VideoPrice480P,
		Price720P:  apiKey.Group.VideoPrice720P,
		Price1080P: apiKey.Group.VideoPrice1080P,
	}
}

func apiKeyHasConfiguredVideoPrice(apiKey *APIKey, resolution string) bool {
	return apiKey != nil && apiKey.Group != nil && apiKey.Group.GetVideoPrice(resolution) != nil
}

func webSearchPricePerCallFromAPIKey(apiKey *APIKey) *float64 {
	if apiKey == nil || apiKey.Group == nil {
		return nil
	}
	return apiKey.Group.WebSearchPricePerCall
}
