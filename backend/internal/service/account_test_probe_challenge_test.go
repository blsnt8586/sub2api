//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

var accountProbeTestQuestionPattern = regexp.MustCompile(`(\d+) ([+-]) (\d+) =`)

type arithmeticProbeHTTPUpstream struct {
	wrongAnswer bool
	lastBody    []byte
}

func (u *arithmeticProbeHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.lastBody = body
	prompt := gjson.GetBytes(body, "input").String()
	matches := accountProbeTestQuestionPattern.FindAllStringSubmatch(prompt, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("probe request does not contain an arithmetic challenge")
	}
	question := matches[len(matches)-1]
	left, _ := strconv.Atoi(question[1])
	right, _ := strconv.Atoi(question[3])
	answer := left + right
	if question[2] == "-" {
		answer = left - right
	}
	if u.wrongAnswer {
		answer++
	}
	stream := fmt.Sprintf("data: {\"type\":\"response.output_text.delta\",\"delta\":\"%d\"}\n\ndata: {\"type\":\"response.completed\"}\n\n", answer)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(stream)),
	}, nil
}

func (u *arithmeticProbeHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func fixedAccountProbeChallengeContext() (context.Context, *accountProbeChallengeState) {
	state := &accountProbeChallengeState{
		Challenge: monitorChallenge{Prompt: "17 + 25 =", Expected: "42"},
	}
	return context.WithValue(context.Background(), accountProbeChallengeContextKey{}, state), state
}

func TestAccountProbeChallengePayloadsRemainStreamingAndBoundOutput(t *testing.T) {
	t.Parallel()

	t.Run("claude", func(t *testing.T) {
		ctx, state := fixedAccountProbeChallengeContext()
		payload, err := createAccountTestClaudePayload(ctx, "claude-test")
		require.NoError(t, err)
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		require.True(t, gjson.GetBytes(encoded, "stream").Bool())
		require.Equal(t, int64(accountProbeChallengeStandardMaxTokens), gjson.GetBytes(encoded, "max_tokens").Int())
		require.Equal(t, state.Challenge.Prompt, gjson.GetBytes(encoded, "messages.0.content.0.text").String())
		require.True(t, state.Applied)
	})

	t.Run("openai_responses", func(t *testing.T) {
		ctx, state := fixedAccountProbeChallengeContext()
		payload := createAccountTestOpenAIResponsesPayload(ctx, "gpt-test", true)
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		require.True(t, gjson.GetBytes(encoded, "stream").Bool())
		require.Equal(t, int64(accountProbeChallengeStandardMaxTokens), gjson.GetBytes(encoded, "max_output_tokens").Int())
		require.Equal(t, state.Challenge.Prompt, gjson.GetBytes(encoded, "input.0.content.0.text").String())
		require.True(t, state.Applied)
	})

	t.Run("openai_chat", func(t *testing.T) {
		ctx, state := fixedAccountProbeChallengeContext()
		payload := createAccountTestOpenAIChatPayload(ctx, "chat-test", "hi")
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		require.True(t, gjson.GetBytes(encoded, "stream").Bool())
		require.Equal(t, int64(accountProbeChallengeStandardMaxTokens), gjson.GetBytes(encoded, "max_tokens").Int())
		require.Equal(t, state.Challenge.Prompt, gjson.GetBytes(encoded, "messages.0.content").String())
		require.True(t, state.Applied)
	})

	t.Run("gemini", func(t *testing.T) {
		ctx, state := fixedAccountProbeChallengeContext()
		payload := createAccountTestGeminiPayload(ctx, "gemini-test", "hi")
		require.Equal(t, int64(accountProbeChallengeStandardMaxTokens), gjson.GetBytes(payload, "generationConfig.maxOutputTokens").Int())
		require.Equal(t, state.Challenge.Prompt, gjson.GetBytes(payload, "contents.0.parts.0.text").String())
		require.True(t, state.Applied)
	})

	t.Run("grok", func(t *testing.T) {
		ctx, state := fixedAccountProbeChallengeContext()
		payload, err := buildAccountTestGrokPayload(ctx, "grok-test")
		require.NoError(t, err)
		require.True(t, gjson.GetBytes(payload, "stream").Bool())
		require.Equal(t, int64(accountProbeChallengeStandardMaxTokens), gjson.GetBytes(payload, "max_output_tokens").Int())
		require.Equal(t, state.Challenge.Prompt, gjson.GetBytes(payload, "input").String())
		require.True(t, state.Applied)
	})

	t.Run("antigravity", func(t *testing.T) {
		ctx, state := fixedAccountProbeChallengeContext()
		payload, err := (&AntigravityGatewayService{}).buildGeminiTestRequest(ctx, "project-1", "gemini-2.5-flash")
		require.NoError(t, err)
		require.Equal(t, int64(accountProbeChallengeTightMaxTokens), gjson.GetBytes(payload, "request.generationConfig.maxOutputTokens").Int())
		require.Equal(t, state.Challenge.Prompt, gjson.GetBytes(payload, "request.contents.0.parts.0.text").String())
		require.True(t, state.Applied)
	})
}

func TestAccountProbeChallengeUsesCompactPromptAndReasoningBudget(t *testing.T) {
	t.Parallel()

	for range 20 {
		challenge := generateCompactAccountProbeChallenge()
		require.Regexp(t, accountProbeTestQuestionPattern, challenge.Prompt)
		require.LessOrEqual(t, len(challenge.Prompt), 12)
	}
	require.Equal(t, accountProbeChallengeReasoningMaxTokens, accountProbeMaxTokensForModel("gpt-5.4"))
	require.Equal(t, accountProbeChallengeReasoningMaxTokens, accountProbeMaxTokensForModel("gemini-3-pro-high"))
	require.Equal(t, accountProbeChallengeReasoningMaxTokens, accountProbeMaxTokensForModel("gemini-3-pro-preview"))
	require.Equal(t, accountProbeChallengeReasoningMaxTokens, accountProbeMaxTokensForModel("deepseek-reasoner"))
	require.Equal(t, accountProbeChallengeStandardMaxTokens, accountProbeMaxTokensForModel("claude-sonnet-4-6"))
	require.Equal(t, accountProbeChallengeStandardMaxTokens, accountProbeMaxTokensForModel("gemini-2.5-flash"))
}

func TestAccountProbeChallengeOmitsRejectedAstraOAuthTokenLimit(t *testing.T) {
	t.Parallel()

	oauthCtx, oauthState := fixedAccountProbeChallengeContext()
	oauthPayload := createAccountTestOpenAIResponsesPayload(oauthCtx, "gpt-6-astra", true)
	require.NotContains(t, oauthPayload, "max_output_tokens")
	require.Equal(t, accountProbeChallengeSystem, oauthPayload["instructions"])
	require.True(t, oauthState.Applied)

	apiKeyCtx, _ := fixedAccountProbeChallengeContext()
	apiKeyPayload := createAccountTestOpenAIResponsesPayload(apiKeyCtx, "gpt-6-astra", false)
	require.Contains(t, apiKeyPayload, "max_output_tokens")

	otherOAuthCtx, _ := fixedAccountProbeChallengeContext()
	otherOAuthPayload := createAccountTestOpenAIResponsesPayload(otherOAuthCtx, "gpt-5.6-sol", true)
	require.Contains(t, otherOAuthPayload, "max_output_tokens")
}

func TestAccountProbeChallengeDoesNotChangeOrdinaryAccountTests(t *testing.T) {
	t.Parallel()

	payload := createAccountTestOpenAIResponsesPayload(context.Background(), "gpt-test", false)
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	require.Equal(t, "hi", gjson.GetBytes(encoded, "input.0.content.0.text").String())
	require.False(t, gjson.GetBytes(encoded, "max_output_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "stream").Bool())
}

func TestAccountProbeChallengeSkipsGeminiMediaPayload(t *testing.T) {
	t.Parallel()

	ctx, state := fixedAccountProbeChallengeContext()
	payload := createAccountTestGeminiPayload(ctx, "gemini-2.5-flash-image", "draw a robot")
	require.Equal(t, "draw a robot", gjson.GetBytes(payload, "contents.0.parts.0.text").String())
	require.False(t, state.Applied)
	require.NoError(t, validateAccountProbeChallenge(state, ""))
}

func TestAccountProbeChallengeSkipsAllMediaModels(t *testing.T) {
	t.Parallel()
	for _, model := range []string{"grok-video-fast", "voice-preview", "sora-2"} {
		ctx, state := fixedAccountProbeChallengeContext()
		payload := createAccountTestOpenAIResponsesPayload(ctx, model, true)
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		require.Equal(t, "hi", gjson.GetBytes(encoded, "input.0.content.0.text").String())
		require.False(t, state.Applied)
	}
}

func TestValidateAccountProbeChallenge(t *testing.T) {
	t.Parallel()

	_, state := fixedAccountProbeChallengeContext()
	state.Applied = true
	require.NoError(t, validateAccountProbeChallenge(state, "42"))
	require.NoError(t, validateAccountProbeChallenge(state, "17 + 25 = 42"))
	require.ErrorContains(t, validateAccountProbeChallenge(state, "17 + 25 ="), "expected 42")
	require.ErrorContains(t, validateAccountProbeChallenge(state, "41"), "expected 42")
	require.ErrorContains(t, validateAccountProbeChallenge(state, ""), "got \"\"")
}

func TestRunProbeTestBackgroundValidatesStreamingArithmeticAnswer(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:          901,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "probe-token"},
	}

	for _, tc := range []struct {
		name        string
		wrongAnswer bool
		wantStatus  string
	}{
		{name: "correct", wantStatus: "success"},
		{name: "wrong", wrongAnswer: true, wantStatus: "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &arithmeticProbeHTTPUpstream{wrongAnswer: tc.wrongAnswer}
			repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
			svc := &AccountTestService{
				accountRepo:         repo,
				httpUpstream:        upstream,
				tlsFPProfileService: &TLSFingerprintProfileService{},
			}

			result, err := svc.RunProbeTestBackground(context.Background(), account.ID, "gpt-5.4")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tc.wantStatus, result.Status)
			require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
			require.Equal(t, int64(accountProbeChallengeReasoningMaxTokens), gjson.GetBytes(upstream.lastBody, "max_output_tokens").Int())
			require.NotEmpty(t, gjson.GetBytes(upstream.lastBody, "input.0.content.0.text").String())
			if tc.wrongAnswer {
				require.Contains(t, result.ErrorMessage, "probe challenge mismatch")
			} else {
				require.Empty(t, result.ErrorMessage)
			}
		})
	}
}
