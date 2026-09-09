//go:build unit

package middleware

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestIsSmartGroupObservedRequest(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/v1/responses", true},
		{http.MethodPost, "/v1/messages", true},
		{http.MethodPost, "/v1/messages/count_tokens", false},
		{http.MethodGet, "/v1/models", false},
		{http.MethodGet, "/v1/usage", false},
		{http.MethodPost, "/v1/images/generations/async", false},
		{http.MethodGet, "/v1/images/tasks/abc", false},
	}
	for _, tt := range tests {
		req, err := http.NewRequest(tt.method, tt.path, nil)
		require.NoError(t, err)
		require.Equal(t, tt.want, isSmartGroupObservedRequest(req), tt.method+" "+tt.path)
	}
}

func TestSmartGroupFailureStatusIncludesUpstreamAuthAndBillingFailures(t *testing.T) {
	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
	} {
		require.True(t, smartGroupFailureStatus(status), "status %d should count as an upstream failure", status)
	}
	for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusNotFound} {
		require.False(t, smartGroupFailureStatus(status), "status %d should not count as an upstream failure", status)
	}
}

func TestSmartGroupCustomAccountErrorStatusIsCarriedByRequestContext(t *testing.T) {
	ctx := service.WithOpsCustomAccountErrorTracking(nil)
	service.MarkOpsCustomAccountError(ctx, 42, http.StatusNotFound)

	accountID, status, matched := service.GetOpsCustomAccountError(ctx)
	require.True(t, matched)
	require.Equal(t, int64(42), accountID)
	require.Equal(t, http.StatusNotFound, status)
}

func TestSmartGroupStreamFailureIncludesExplicitCustomAccountError(t *testing.T) {
	customStreamError := []service.OpsStreamError{{
		Message:         "custom upstream 404",
		IntendedStatus:  http.StatusNotFound,
		CountTowardsSLA: true,
	}}

	message, failed := smartGroupStreamFailure(customStreamError, true)
	require.True(t, failed)
	require.Equal(t, "custom upstream 404", message)

	_, failed = smartGroupStreamFailure(customStreamError, false)
	require.False(t, failed, "ordinary 404 must not become a smart-group failure")

	customStreamError[0].CountTowardsSLA = false
	_, failed = smartGroupStreamFailure(customStreamError, true)
	require.False(t, failed, "an intermediate error followed by success must not count")
}
