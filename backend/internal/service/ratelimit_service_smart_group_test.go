//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRateLimitServiceMarksExplicitCustomErrorForSmartGroup(t *testing.T) {
	repo := &errorPolicyRepoStub{}
	svc := NewRateLimitService(repo, nil, nil, nil, nil)
	account := &Account{
		ID:       501,
		Type:     AccountTypeAPIKey,
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(http.StatusBadRequest), float64(http.StatusNotFound)},
		},
	}
	ctx := WithOpsCustomAccountErrorTracking(context.Background())

	require.True(t, svc.HandleUpstreamError(ctx, account, http.StatusNotFound, http.Header{}, []byte(`{"error":{"message":"not found"}}`)))
	accountID, status, matched := GetOpsCustomAccountError(ctx)
	require.True(t, matched)
	require.Equal(t, int64(account.ID), accountID)
	require.Equal(t, http.StatusNotFound, status)
}

func TestRateLimitServiceDoesNotMarkCustomErrorWhenStatusIsNotSelected(t *testing.T) {
	repo := &errorPolicyRepoStub{}
	svc := NewRateLimitService(repo, nil, nil, nil, nil)
	account := &Account{
		ID:       502,
		Type:     AccountTypeAPIKey,
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(http.StatusNotFound)},
		},
	}
	ctx := WithOpsCustomAccountErrorTracking(context.Background())

	require.False(t, svc.HandleUpstreamError(ctx, account, http.StatusBadRequest, http.Header{}, []byte(`{"error":{"message":"bad request"}}`)))
	_, _, matched := GetOpsCustomAccountError(ctx)
	require.False(t, matched)
}

func TestRateLimitServiceCheckErrorPolicyMarksSelectedCustomError(t *testing.T) {
	account := &Account{
		ID:       503,
		Type:     AccountTypeAPIKey,
		Platform: PlatformGemini,
		Credentials: map[string]any{
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(http.StatusBadRequest)},
		},
	}
	ctx := WithOpsCustomAccountErrorTracking(context.Background())
	svc := NewRateLimitService(&errorPolicyRepoStub{}, nil, nil, nil, nil)

	require.Equal(t, ErrorPolicyMatched, svc.CheckErrorPolicy(ctx, account, http.StatusBadRequest, []byte("bad request")))
	accountID, status, matched := GetOpsCustomAccountError(ctx)
	require.True(t, matched)
	require.Equal(t, int64(account.ID), accountID)
	require.Equal(t, http.StatusBadRequest, status)
}

func TestGrokExplicitCustomErrorOverridesRequestScopedShortcut(t *testing.T) {
	repo := &errorPolicyRepoStub{}
	rateLimitService := NewRateLimitService(repo, nil, nil, nil, nil)
	gateway := &OpenAIGatewayService{rateLimitService: rateLimitService}
	account := &Account{
		ID:       504,
		Type:     AccountTypeAPIKey,
		Platform: PlatformGrok,
		Credentials: map[string]any{
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(http.StatusForbidden)},
		},
	}
	ctx := WithOpsCustomAccountErrorTracking(context.Background())

	gateway.handleGrokAccountUpstreamError(
		ctx,
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"code":"new_sensitive","message":"request blocked"}}`),
	)

	require.Equal(t, 1, repo.setErrCalls)
	accountID, status, matched := GetOpsCustomAccountError(ctx)
	require.True(t, matched)
	require.Equal(t, int64(account.ID), accountID)
	require.Equal(t, http.StatusForbidden, status)
}

func TestAntigravityExplicitCustomErrorReachesAccountPolicy(t *testing.T) {
	repo := &errorPolicyRepoStub{}
	rateLimitService := NewRateLimitService(repo, nil, nil, nil, nil)
	gateway := &AntigravityGatewayService{rateLimitService: rateLimitService}
	account := &Account{
		ID:       505,
		Type:     AccountTypeAPIKey,
		Platform: PlatformAntigravity,
		Credentials: map[string]any{
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(http.StatusNotFound)},
		},
	}
	ctx := WithOpsCustomAccountErrorTracking(context.Background())

	gateway.handleUpstreamError(
		ctx,
		"test",
		account,
		http.StatusNotFound,
		http.Header{},
		[]byte(`{"error":{"message":"not found"}}`),
		"gemini-2.5-pro",
		0,
		"",
		false,
	)

	require.Equal(t, 1, repo.setErrCalls)
	accountID, status, matched := GetOpsCustomAccountError(ctx)
	require.True(t, matched)
	require.Equal(t, int64(account.ID), accountID)
	require.Equal(t, http.StatusNotFound, status)
}

func TestOpenAIStreamDeclaredStatusPreservesCustomHTTPCode(t *testing.T) {
	for _, payload := range []string{
		`{"response":{"error":{"status_code":404}}}`,
		`{"error":{"status":400}}`,
		`{"status_code":418}`,
	} {
		require.NotZero(t, openAIStreamDeclaredStatus([]byte(payload)))
	}
	require.Zero(t, openAIStreamDeclaredStatus([]byte(`{"response":{"status":"failed"}}`)))
}

func TestAccountCustomErrorCodesAcceptTypedAndStringValues(t *testing.T) {
	account := &Account{Credentials: map[string]any{
		"custom_error_codes_enabled": true,
		"custom_error_codes":         []int{404, 404, 400},
	}}
	require.Equal(t, []int{400, 404}, account.GetCustomErrorCodes())
	require.True(t, account.ShouldHandleErrorCode(http.StatusNotFound))

	account.Credentials["custom_error_codes"] = []any{" 403 ", json.Number("429")}
	require.Equal(t, []int{403, 429}, account.GetCustomErrorCodes())
}
