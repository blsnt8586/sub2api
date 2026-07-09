package domain

// AllPlatforms 所有支持的平台（单一权威列表）。
// 新增平台时只需在此追加一行，handler 验证与 schema 约束自动同步——无需修改上游文件。
var AllPlatforms = []string{
	PlatformAnthropic,
	PlatformOpenAI,
	PlatformGemini,
	PlatformAntigravity,
	PlatformGrok,
	PlatformJimeng,
}

// VideoPlatforms 支持视频生成 API 的平台。
// 新增视频平台时在此追加，gateway_video.go 中补充对应 handler 即可。
var VideoPlatforms = []string{
	PlatformGrok,
	PlatformJimeng,
}

// IsValidPlatform 报告 p 是否为合法的平台标识。
func IsValidPlatform(p string) bool {
	for _, ap := range AllPlatforms {
		if p == ap {
			return true
		}
	}
	return false
}

// PlatformSupportsVideo 报告平台是否支持视频生成 API。
func PlatformSupportsVideo(p string) bool {
	for _, vp := range VideoPlatforms {
		if p == vp {
			return true
		}
	}
	return false
}
