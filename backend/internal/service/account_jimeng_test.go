//go:build unit

package service

import "testing"

func TestAccountIsJimeng(t *testing.T) {
	if !(&Account{Platform: PlatformCanvas}).IsCanvas() {
		t.Error("jimeng account should report IsJimeng true")
	}
	if (&Account{Platform: PlatformGrok}).IsCanvas() {
		t.Error("grok account should report IsJimeng false")
	}
}

func TestAccountGetCanvasBaseURL(t *testing.T) {
	// 显式配置的 base_url 原样返回
	acc := &Account{Platform: PlatformCanvas, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://zz1cc.cc.cd/v1"}}
	if got := acc.GetCanvasBaseURL(); got != "https://zz1cc.cc.cd/v1" {
		t.Errorf("GetCanvasBaseURL = %q, want https://zz1cc.cc.cd/v1", got)
	}

	// 未配置时兜底默认
	accNoBase := &Account{Platform: PlatformCanvas, Type: AccountTypeAPIKey, Credentials: map[string]any{}}
	if got := accNoBase.GetCanvasBaseURL(); got == "" {
		t.Error("GetCanvasBaseURL should fall back to a non-empty default")
	}

	// 非即梦账号返回空
	notJimeng := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://x"}}
	if got := notJimeng.GetCanvasBaseURL(); got != "" {
		t.Errorf("non-jimeng GetCanvasBaseURL = %q, want empty", got)
	}
}

func TestAccountGetCanvasAPIKey(t *testing.T) {
	acc := &Account{Platform: PlatformCanvas, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": " jm-secret "}}
	if got := acc.GetCanvasAPIKey(); got != "jm-secret" {
		t.Errorf("GetCanvasAPIKey = %q, want jm-secret (trimmed)", got)
	}
	notJimeng := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "x"}}
	if got := notJimeng.GetCanvasAPIKey(); got != "" {
		t.Errorf("non-jimeng GetCanvasAPIKey = %q, want empty", got)
	}
}
