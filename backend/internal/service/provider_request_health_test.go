//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderHealthFailureClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", want: false},
		{name: "client cancellation", err: context.Canceled, want: false},
		{name: "request timeout", err: context.DeadlineExceeded, want: true},
		{name: "upstream server error", err: &UpstreamFailoverError{StatusCode: 502}, want: true},
		{name: "client error", err: &UpstreamFailoverError{StatusCode: 400}, want: false},
		{name: "account capability mismatch is not an outage", err: newOpenAIResponsesToolsRequireResponsesFailoverError("gpt-6.1-sol"), want: false},
		{name: "rate limit", err: &UpstreamFailoverError{StatusCode: 429}, want: false},
		{name: "request scoped capacity", err: &UpstreamFailoverError{StatusCode: 529, RequestScopedTransient: true}, want: false},
		{name: "provider scoped error", err: &UpstreamFailoverError{StatusCode: 502, Scope: GatewayFailureScopeProvider}, want: false},
		{name: "request scoped error", err: &UpstreamFailoverError{StatusCode: 502, Scope: GatewayFailureScopeRequest}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, providerHealthFailure(tc.err))
		})
	}
}
