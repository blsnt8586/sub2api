package domain

const (
	Sub2APIProviderAuthModePassword    = "password"
	Sub2APIProviderAuthModeTokenPair   = "token_pair"
	Sub2APIProviderAuthModeAccessToken = "access_token"
)

func IsValidSub2APIProviderAuthMode(mode string) bool {
	switch mode {
	case Sub2APIProviderAuthModePassword, Sub2APIProviderAuthModeTokenPair, Sub2APIProviderAuthModeAccessToken:
		return true
	default:
		return false
	}
}

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
// 支持 Sub2API 与 New API 控制面协议。
const (
	ProviderTypeSub2API = "sub2api"
	ProviderTypeNewAPI  = "newapi"

	// ProviderTypeDefault 是未显式指定时的默认上游类型。
	ProviderTypeDefault = ProviderTypeSub2API
)

// IsValidProviderType 判断上游类型是否受支持。
func IsValidProviderType(t string) bool {
	switch t {
	case ProviderTypeSub2API, ProviderTypeNewAPI:
		return true
	default:
		return false
	}
}
