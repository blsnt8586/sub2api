package domain

// Sub2API Provider 相关常量

const (
	// Platform 类型
	ProviderPlatformAnthropic = "anthropic"
	ProviderPlatformOpenAI    = "openai"
	ProviderPlatformGemini    = "gemini"

	// 同步状态
	ProviderSyncStatusSuccess = "success"
	ProviderSyncStatusFailed  = "failed"
	ProviderSyncStatusPending = "pending"
)

// 上游类型（provider_type）：标识上游实例的协议/接口类型，决定同步、登录、分组等接口逻辑走哪套实现。
// 与「账号平台」(anthropic/openai/gemini) 语义不同——一个上游实例下可挂多种平台的账号。
// 当前仅支持 sub2api，后续扩展其他上游协议时在此追加，并在 IsValidProviderType 中放行。
const (
	ProviderTypeSub2API = "sub2api"

	// ProviderTypeDefault 是未显式指定时的默认上游类型。
	ProviderTypeDefault = ProviderTypeSub2API
)

// 登录方式（login_method）：决定登录时走普通 HTTP 还是浏览器（用于绕过 Cloudflare Turnstile）。
const (
	// LoginMethodHTTP 直接 HTTP POST 登录（默认，适用于大多数平台）
	LoginMethodHTTP = "http"
	// LoginMethodBrowser 通过 undetected-chromedriver 浏览器登录（适用于有 Turnstile 验证的平台）
	LoginMethodBrowser = "browser"
	// LoginMethodDefault 默认登录方式
	LoginMethodDefault = LoginMethodHTTP
)

// IsValidLoginMethod 判断登录方式是否合法
func IsValidLoginMethod(m string) bool {
	switch m {
	case LoginMethodHTTP, LoginMethodBrowser:
		return true
	default:
		return false
	}
}

// IsValidProviderType 判断上游类型是否受支持。
func IsValidProviderType(t string) bool {
	switch t {
	case ProviderTypeSub2API:
		return true
	default:
		return false
	}
}
