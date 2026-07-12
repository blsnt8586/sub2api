package service

// 本文件为二开新增：OpenAI/Codex 全局 system prompt 注入 helper 的单元测试。

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestInjectOpenAIGlobalInstructions(t *testing.T) {
	const prompt = "禁止讨论竞品"

	t.Run("empty prompt returns body unchanged", func(t *testing.T) {
		body := []byte(`{"instructions":"你是客服"}`)
		got, changed := injectOpenAIGlobalInstructions(body, "")
		if changed {
			t.Fatalf("expected changed=false for empty prompt")
		}
		if string(got) != string(body) {
			t.Fatalf("body should be unchanged, got %s", got)
		}
	})

	t.Run("whitespace-only prompt returns body unchanged", func(t *testing.T) {
		body := []byte(`{"instructions":"你是客服"}`)
		_, changed := injectOpenAIGlobalInstructions(body, "   \n\t ")
		if changed {
			t.Fatalf("expected changed=false for whitespace-only prompt")
		}
	})

	t.Run("no existing instructions sets prompt directly", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.1","input":[]}`)
		got, changed := injectOpenAIGlobalInstructions(body, prompt)
		if !changed {
			t.Fatalf("expected changed=true")
		}
		if v := gjson.GetBytes(got, "instructions").String(); v != prompt {
			t.Fatalf("instructions=%q, want %q", v, prompt)
		}
		// 其余字段保持不变
		if v := gjson.GetBytes(got, "model").String(); v != "gpt-5.1" {
			t.Fatalf("model mutated: %q", v)
		}
	})

	t.Run("empty existing instructions sets prompt directly", func(t *testing.T) {
		body := []byte(`{"instructions":"   "}`)
		got, changed := injectOpenAIGlobalInstructions(body, prompt)
		if !changed {
			t.Fatalf("expected changed=true")
		}
		if v := gjson.GetBytes(got, "instructions").String(); v != prompt {
			t.Fatalf("instructions=%q, want %q", v, prompt)
		}
	})

	t.Run("existing instructions are prepended-merged", func(t *testing.T) {
		body := []byte(`{"instructions":"你是客服机器人"}`)
		got, changed := injectOpenAIGlobalInstructions(body, prompt)
		if !changed {
			t.Fatalf("expected changed=true")
		}
		want := prompt + "\n\n你是客服机器人"
		if v := gjson.GetBytes(got, "instructions").String(); v != want {
			t.Fatalf("instructions=%q, want %q", v, want)
		}
	})

	t.Run("idempotent: already-prefixed body is not re-injected", func(t *testing.T) {
		body := []byte(`{"instructions":"你是客服机器人"}`)
		once, _ := injectOpenAIGlobalInstructions(body, prompt)
		twice, changed := injectOpenAIGlobalInstructions(once, prompt)
		if changed {
			t.Fatalf("expected changed=false on second injection")
		}
		if string(twice) != string(once) {
			t.Fatalf("second injection mutated body:\n once=%s\ntwice=%s", once, twice)
		}
	})
}
