//go:build unit

package service

import "testing"

// TestSettingKeyDefaultPlatformQuotas 验证新的系统层 JSON key 常量值正确。
func TestSettingKeyDefaultPlatformQuotas(t *testing.T) {
	if SettingKeyDefaultPlatformQuotas != "default_platform_quotas" {
		t.Errorf("SettingKeyDefaultPlatformQuotas = %q, want %q",
			SettingKeyDefaultPlatformQuotas, "default_platform_quotas")
	}
}

// TestSettingKeyAuthSourcePlatformQuotas 验证新的 auth-source JSON key 函数返回值正确。
func TestSettingKeyAuthSourcePlatformQuotas(t *testing.T) {
	if got := SettingKeyAuthSourcePlatformQuotas("email"); got != "auth_source_default_email_platform_quotas" {
		t.Fatalf("got %q, want %q", got, "auth_source_default_email_platform_quotas")
	}
	if got := SettingKeyAuthSourcePlatformQuotas("dingtalk"); got != "auth_source_default_dingtalk_platform_quotas" {
		t.Fatalf("got %q, want %q", got, "auth_source_default_dingtalk_platform_quotas")
	}
}

// TestPlatformJimeng_Constant 验证即梦平台常量值正确。
func TestPlatformJimeng_Constant(t *testing.T) {
	if PlatformJimeng != "jimeng" {
		t.Errorf("PlatformJimeng = %q, want %q", PlatformJimeng, "jimeng")
	}
}

// TestAllowedQuotaPlatforms_ContainsJimeng 验证 AllowedQuotaPlatforms 包含即梦平台。
func TestAllowedQuotaPlatforms_ContainsJimeng(t *testing.T) {
	found := false
	for _, p := range AllowedQuotaPlatforms {
		if p == PlatformJimeng {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AllowedQuotaPlatforms 应包含 %q，当前: %v", PlatformJimeng, AllowedQuotaPlatforms)
	}
}

// TestIsAllowedQuotaPlatform_Jimeng 验证 IsAllowedQuotaPlatform 对即梦返回 true。
func TestIsAllowedQuotaPlatform_Jimeng(t *testing.T) {
	if !IsAllowedQuotaPlatform(PlatformJimeng) {
		t.Errorf("IsAllowedQuotaPlatform(%q) = false, want true", PlatformJimeng)
	}
}
