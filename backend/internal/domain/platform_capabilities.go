package domain

// AllPlatforms 所有支持的平台（单一权威列表）。
// 新增平台时只需在此追加一行，handler 验证与 schema 约束自动同步——无需修改上游文件。
var AllPlatforms = []string{
	PlatformAnthropic,
	PlatformOpenAI,
	PlatformGemini,
	PlatformAntigravity,
	PlatformGrok,
	PlatformCanvas,
}

// VideoPlatforms 支持视频生成 API 的平台。
// 新增视频平台时在此追加，gateway_video.go 中补充对应 handler 即可。
var VideoPlatforms = []string{
	PlatformGrok,
	PlatformCanvas,
}

// GroupPlatforms 允许作为分组（group.platform）取值的平台列表。
// 比 AllPlatforms 多一个 composite：复合分组把多个平台的账号挂在同一分组下，
// 按 model/endpoint 路由到具体平台。composite 只是分组的调度模式，不是账号平台，
// 因此不进 AllPlatforms（否则会多出一个无意义的账号平台与 user×platform 配额维度）。
var GroupPlatforms = append(append([]string{}, AllPlatforms...), PlatformComposite)

// IsValidPlatform 报告 p 是否为合法的平台标识（账号平台，不含 composite）。
func IsValidPlatform(p string) bool {
	for _, ap := range AllPlatforms {
		if p == ap {
			return true
		}
	}
	return false
}

// IsValidGroupPlatform 报告 p 是否为合法的分组平台标识（含 composite）。
func IsValidGroupPlatform(p string) bool {
	for _, gp := range GroupPlatforms {
		if p == gp {
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
