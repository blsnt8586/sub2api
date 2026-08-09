package service

// 本文件为二开新增：OpenAI/Codex 全局 system prompt 注入。
// 与上游解耦——读取缓存、注入逻辑全部自包含在此文件，
// 上游文件仅有极少量 hook 调用。详见 CUSTOM-CHANGES.md。

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"golang.org/x/sync/singleflight"
)

// cachedOpenAISystemPromptInjection 缓存 OpenAI 全局 system prompt 注入设置（进程内，60s TTL）。
// 网关热路径每请求读取一次，通过 atomic.Value 零锁访问，避免每次访问 DB。
type cachedOpenAISystemPromptInjection struct {
	enabled   bool
	prompt    string
	expiresAt int64 // unix nano
}

var openAISystemPromptInjectionCache atomic.Value // *cachedOpenAISystemPromptInjection
var openAISystemPromptInjectionSF singleflight.Group

const openAISystemPromptInjectionCacheTTL = 60 * time.Second
const openAISystemPromptInjectionErrorTTL = 5 * time.Second
const openAISystemPromptInjectionDBTimeout = 5 * time.Second

// GetOpenAISystemPromptInjection 返回 OpenAI/Codex 全局 system prompt 注入设置。
// 进程内 atomic.Value 缓存（60s TTL）+ singleflight 防击穿，网关热路径零锁读取。
// 默认：关闭（opt-in），prompt 空。
func (s *SettingService) GetOpenAISystemPromptInjection(ctx context.Context) (enabled bool, prompt string) {
	if s == nil || s.settingRepo == nil {
		return false, ""
	}
	if cached, ok := openAISystemPromptInjectionCache.Load().(*cachedOpenAISystemPromptInjection); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.enabled, cached.prompt
		}
	}
	result, _, _ := openAISystemPromptInjectionSF.Do("openai_system_prompt_injection", func() (any, error) {
		if cached, ok := openAISystemPromptInjectionCache.Load().(*cachedOpenAISystemPromptInjection); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAISystemPromptInjectionDBTimeout)
		defer cancel()
		values, err := s.settingRepo.GetMultiple(dbCtx, []string{
			SettingKeyEnableOpenAISystemPromptInjection,
			SettingKeyOpenAISystemPrompt,
		})
		if err != nil {
			slog.Warn("failed to get openai system prompt injection settings", "error", err)
			entry := &cachedOpenAISystemPromptInjection{
				enabled:   false,
				prompt:    "",
				expiresAt: time.Now().Add(openAISystemPromptInjectionErrorTTL).UnixNano(),
			}
			openAISystemPromptInjectionCache.Store(entry)
			return entry, nil
		}
		entry := &cachedOpenAISystemPromptInjection{
			enabled:   values[SettingKeyEnableOpenAISystemPromptInjection] == "true",
			prompt:    values[SettingKeyOpenAISystemPrompt],
			expiresAt: time.Now().Add(openAISystemPromptInjectionCacheTTL).UnixNano(),
		}
		openAISystemPromptInjectionCache.Store(entry)
		return entry, nil
	})
	if entry, ok := result.(*cachedOpenAISystemPromptInjection); ok && entry != nil {
		return entry.enabled, entry.prompt
	}
	return false, ""
}

// injectOpenAIGlobalInstructions 把全局 system prompt 前置合并到 Responses 格式 body 的
// 顶层 instructions 字段（Codex/responses 与 chat→responses 两条路径统一走此字段）。
//   - prompt 为空：原样返回，changed=false。
//   - 已有 instructions：合并为 prompt + "\n\n" + 原内容（保留客户端指令，全局规则在前）。
//   - 无 instructions：直接设为 prompt。
//
// 幂等：若 instructions 已以 prompt 开头则跳过，避免重试/多次调用重复注入。
// 返回 changed 标记 body 是否被改写，供调用方决定是否重置下游解析状态。
func injectOpenAIGlobalInstructions(body []byte, prompt string) (result []byte, changed bool) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return body, false
	}
	existingStr := ""
	if existing := gjson.GetBytes(body, "instructions"); existing.Exists() && existing.Type == gjson.String {
		existingStr = existing.String()
	}
	// 幂等守卫：已前置过则不再重复注入。
	if strings.HasPrefix(strings.TrimSpace(existingStr), trimmed) {
		return body, false
	}
	merged := trimmed
	if strings.TrimSpace(existingStr) != "" {
		merged = trimmed + "\n\n" + existingStr
	}
	if next, err := sjson.SetBytes(body, "instructions", merged); err == nil {
		return next, true
	}
	return body, false
}
