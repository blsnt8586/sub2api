//go:build unit

package service

import "testing"

func TestAccountIsJimeng(t *testing.T) {
	if !(&Account{Platform: PlatformJimeng}).IsJimeng() {
		t.Error("jimeng account should report IsJimeng true")
	}
	if (&Account{Platform: PlatformGrok}).IsJimeng() {
		t.Error("grok account should report IsJimeng false")
	}
}

func TestAccountGetJimengBaseURL(t *testing.T) {
	// 显式配置的 base_url 原样返回
	acc := &Account{Platform: PlatformJimeng, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://zz1cc.cc.cd/v1"}}
	if got := acc.GetJimengBaseURL(); got != "https://zz1cc.cc.cd/v1" {
		t.Errorf("GetJimengBaseURL = %q, want https://zz1cc.cc.cd/v1", got)
	}

	// 未配置时兜底默认
	accNoBase := &Account{Platform: PlatformJimeng, Type: AccountTypeAPIKey, Credentials: map[string]any{}}
	if got := accNoBase.GetJimengBaseURL(); got == "" {
		t.Error("GetJimengBaseURL should fall back to a non-empty default")
	}

	// 非即梦账号返回空
	notJimeng := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://x"}}
	if got := notJimeng.GetJimengBaseURL(); got != "" {
		t.Errorf("non-jimeng GetJimengBaseURL = %q, want empty", got)
	}
}

func TestAccountGetJimengAPIKey(t *testing.T) {
	acc := &Account{Platform: PlatformJimeng, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": " jm-secret "}}
	if got := acc.GetJimengAPIKey(); got != "jm-secret" {
		t.Errorf("GetJimengAPIKey = %q, want jm-secret (trimmed)", got)
	}
	notJimeng := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "x"}}
	if got := notJimeng.GetJimengAPIKey(); got != "" {
		t.Errorf("non-jimeng GetJimengAPIKey = %q, want empty", got)
	}
}
