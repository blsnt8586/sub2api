package service

import "context"

type accountTestStatusMutationContextKey struct{}

// suppressAccountTestStatusMutation marks a request as owned by the Provider
// probe state machine. Account tests still perform the real upstream request,
// but they do not mutate the Account row or scheduling state themselves.
func suppressAccountTestStatusMutation(ctx context.Context) context.Context {
	return context.WithValue(ctx, accountTestStatusMutationContextKey{}, true)
}

func accountTestStatusMutationAllowed(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	suppressed, _ := ctx.Value(accountTestStatusMutationContextKey{}).(bool)
	return !suppressed
}

// Sub2APIProbeTestRunner is the narrow account-test capability required by a
// Provider route probe. Keeping this port separate from AccountTestService
// means the Provider control plane does not need to depend on the upstream
// account-test implementation in tests or future adapters.
type Sub2APIProbeTestRunner interface {
	RunProbeTestBackground(context.Context, int64, string) (*ScheduledTestResult, error)
}

// Sub2APIOptimizeTestRunner is the account-test capability required by the
// Provider group optimizer. The optimizer can run either an ordinary test or a
// probe-owned test, depending on the context supplied by the caller.
type Sub2APIOptimizeTestRunner interface {
	RunTestBackground(context.Context, int64, string) (*ScheduledTestResult, error)
	Sub2APIProbeTestRunner
}

var _ Sub2APIProbeTestRunner = (*AccountTestService)(nil)
var _ Sub2APIOptimizeTestRunner = (*AccountTestService)(nil)
