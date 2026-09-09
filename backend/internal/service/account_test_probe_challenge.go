package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

const (
	accountProbeChallengeStandardMaxTokens  = 16
	accountProbeChallengeTightMaxTokens     = 8
	accountProbeChallengeReasoningMaxTokens = 50
	accountProbeChallengeSystem             = "Answer only."
	accountProbeResponsePreviewMax          = 160
)

type accountProbeChallengeContextKey struct{}

// accountProbeChallengeState is request-local. Payload builders mark Applied
// only when they actually replace a text prompt, so media probes keep their
// existing behavior and are not incorrectly validated as arithmetic replies.
type accountProbeChallengeState struct {
	Challenge monitorChallenge
	Applied   bool
}

func withAccountProbeChallenge(ctx context.Context) (context.Context, *accountProbeChallengeState) {
	if ctx == nil {
		ctx = context.Background()
	}
	state := &accountProbeChallengeState{Challenge: generateCompactAccountProbeChallenge()}
	return context.WithValue(ctx, accountProbeChallengeContextKey{}, state), state
}

// generateCompactAccountProbeChallenge keeps the same randomized arithmetic
// proof as channel monitoring without its two few-shot examples. Account probes
// run much more frequently, so reducing the challenge to the equation itself
// saves roughly 40+ input tokens per text request while preserving validation.
func generateCompactAccountProbeChallenge() monitorChallenge {
	a := randIntInRange(monitorChallengeMin, monitorChallengeMax)
	b := randIntInRange(monitorChallengeMin, monitorChallengeMax)
	operator := "+"
	answer := a + b
	if rand.IntN(2) != 0 { //nolint:gosec // Availability challenge, not a security token.
		operator = "-"
		if b > a {
			a, b = b, a
		}
		answer = a - b
	}
	return monitorChallenge{
		Prompt:   fmt.Sprintf("%d %s %d =", a, operator, b),
		Expected: strconv.Itoa(answer),
	}
}

func accountProbePrompt(ctx context.Context, fallback string) (string, int, bool) {
	state, ok := accountProbeChallengeFromContext(ctx)
	if !ok {
		return fallback, 0, false
	}
	state.Applied = true
	return state.Challenge.Prompt, accountProbeMaxTokensForModel(""), true
}

func accountProbePromptForModel(ctx context.Context, modelID, fallback string) (string, int, bool) {
	// Media probes have their own request/response contract. They must not be
	// converted into arithmetic requests or validated as text responses.
	if isProviderProbeMediaModel(modelID) {
		return fallback, 0, false
	}
	prompt, _, applied := accountProbePrompt(ctx, fallback)
	return prompt, accountProbeMaxTokensForModel(modelID), applied
}

func accountProbeMaxTokensForModel(modelID string) int {
	model := strings.ToLower(strings.TrimSpace(modelID))
	if strings.HasPrefix(model, "gpt-5") ||
		strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o3") ||
		strings.HasPrefix(model, "o4") ||
		strings.Contains(model, "reasoning") ||
		strings.Contains(model, "reasoner") ||
		strings.Contains(model, "thinking") ||
		strings.HasPrefix(model, "gemini-3") ||
		strings.Contains(model, "-high") ||
		strings.Contains(model, "-medium") ||
		strings.Contains(model, "-low") {
		return accountProbeChallengeReasoningMaxTokens
	}
	return accountProbeChallengeStandardMaxTokens
}

func accountProbeChallengeFromContext(ctx context.Context) (*accountProbeChallengeState, bool) {
	if ctx == nil {
		return nil, false
	}
	state, ok := ctx.Value(accountProbeChallengeContextKey{}).(*accountProbeChallengeState)
	return state, ok && state != nil
}

func validateAccountProbeChallenge(state *accountProbeChallengeState, responseText string) error {
	if state == nil || !state.Applied {
		return nil
	}
	if validateCompactAccountProbeAnswer(responseText, state.Challenge.Expected) {
		return nil
	}
	preview := strings.TrimSpace(responseText)
	if len(preview) > accountProbeResponsePreviewMax {
		preview = preview[:accountProbeResponsePreviewMax] + "..."
	}
	return fmt.Errorf("probe challenge mismatch: expected %s, got %q", state.Challenge.Expected, preview)
}

func validateCompactAccountProbeAnswer(responseText, expected string) bool {
	if responseText == "" || expected == "" {
		return false
	}
	matches := monitorChallengeNumberRegex.FindAllString(responseText, -1)
	return len(matches) > 0 && matches[len(matches)-1] == expected
}

func createAccountTestClaudePayload(ctx context.Context, modelID string) (map[string]any, error) {
	payload, err := createTestPayload(modelID)
	if err != nil {
		return nil, err
	}
	prompt, maxTokens, probe := accountProbePromptForModel(ctx, modelID, "hi")
	if !probe {
		return payload, nil
	}
	// Preserve the small canonical Claude Code identity system block. OAuth
	// subscription routes use it to verify the client shape; dropping it would
	// make the probe cheaper but less representative and can cause false reds.
	messages := payload["messages"].([]map[string]any)
	content := messages[0]["content"].([]map[string]any)
	content[0]["text"] = prompt
	payload["max_tokens"] = maxTokens
	return payload, nil
}

func createAccountTestOpenAIResponsesPayload(ctx context.Context, modelID string, isOAuth bool) map[string]any {
	payload := createOpenAITestPayload(modelID, isOAuth)
	prompt, maxTokens, probe := accountProbePromptForModel(ctx, modelID, "hi")
	if !probe {
		return payload
	}
	input := payload["input"].([]map[string]any)
	content := input[0]["content"].([]map[string]any)
	content[0]["text"] = prompt
	payload["instructions"] = accountProbeChallengeSystem
	// [CUSTOM][TEMP-UPSTREAM-COMPAT] ChatGPT Codex 的 gpt-6-astra 当前会以
	// `Unsupported parameter: max_output_tokens` 拒绝该字段。账号测试只发送一道
	// 极短算术题，Astra OAuth 探测省略这个不兼容字段即可，其他平台/模型继续
	// 保留低成本 token 上限。上游账号测试接入通用 rejected-field retry 后删除。
	if !(isOAuth && isOpenAIGPT6AstraModel(modelID)) {
		payload["max_output_tokens"] = maxTokens
	}
	return payload
}

func createAccountTestOpenAIChatPayload(ctx context.Context, modelID, prompt string) map[string]any {
	probePrompt, maxTokens, probe := accountProbePromptForModel(ctx, modelID, prompt)
	if !probe {
		return createOpenAIChatCompletionsTestPayload(modelID, prompt)
	}
	payload := createOpenAIChatCompletionsTestPayload(modelID, probePrompt)
	payload["max_tokens"] = maxTokens
	return payload
}

func createAccountTestGeminiPayload(ctx context.Context, modelID, prompt string) []byte {
	// Media generation cannot prove an arithmetic response. Do not mark the
	// challenge as applied; the caller will retain the existing media probe.
	if isProviderProbeMediaModel(modelID) || isImageGenerationModel(modelID) {
		return createGeminiTestPayload(modelID, prompt)
	}
	probePrompt, maxTokens, probe := accountProbePromptForModel(ctx, modelID, prompt)
	if !probe {
		return createGeminiTestPayload(modelID, prompt)
	}
	payload := createGeminiTestPayload(modelID, probePrompt)
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return payload
	}
	generationConfig, _ := body["generationConfig"].(map[string]any)
	if generationConfig == nil {
		generationConfig = make(map[string]any)
		body["generationConfig"] = generationConfig
	}
	generationConfig["maxOutputTokens"] = maxTokens
	delete(body, "systemInstruction")
	encoded, err := json.Marshal(body)
	if err != nil {
		return payload
	}
	return encoded
}

func buildAccountTestGrokPayload(ctx context.Context, modelID string) ([]byte, error) {
	prompt, maxTokens, probe := accountProbePromptForModel(ctx, modelID, grokQuotaProbeInput)
	if !probe {
		return buildGrokQuotaProbeBody(modelID)
	}
	return json.Marshal(map[string]any{
		"model":             modelID,
		"input":             prompt,
		"instructions":      accountProbeChallengeSystem,
		"max_output_tokens": maxTokens,
		"stream":            true,
	})
}
