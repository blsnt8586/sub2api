package service

import (
	"context"
	"sync"
)

// opsCustomAccountErrorTracker records an account-level custom error that was
// actually matched while serving the current request. It lives behind a
// pointer in the request context because gateway services receive derived
// context.Context values rather than the original gin.Context.
type opsCustomAccountErrorTracker struct {
	mu        sync.RWMutex
	matched   bool
	accountID int64
	status    int
}

type opsCustomAccountErrorTrackerKey struct{}

func isConfiguredCustomAccountError(account *Account, statusCode int) bool {
	return account != nil && account.IsCustomErrorCodesEnabled() &&
		len(account.GetCustomErrorCodes()) > 0 && account.ShouldHandleErrorCode(statusCode)
}

// WithOpsCustomAccountErrorTracking installs the request-local tracker used to
// carry account custom-error matches from gateway services to API-key
// middleware. Calling it more than once preserves the existing tracker.
func WithOpsCustomAccountErrorTracking(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(opsCustomAccountErrorTrackerKey{}).(*opsCustomAccountErrorTracker); ok {
		return ctx
	}
	return context.WithValue(ctx, opsCustomAccountErrorTrackerKey{}, &opsCustomAccountErrorTracker{})
}

// MarkOpsCustomAccountError records that the account's explicitly configured
// custom error policy matched this upstream status. It is intentionally a
// no-op for contexts that are not serving an API-key request, which keeps the
// account and rate-limit services usable in background jobs and unit tests.
func MarkOpsCustomAccountError(ctx context.Context, accountID int64, statusCode int) {
	if ctx == nil || accountID <= 0 || statusCode < 100 || statusCode > 599 {
		return
	}
	tracker, _ := ctx.Value(opsCustomAccountErrorTrackerKey{}).(*opsCustomAccountErrorTracker)
	if tracker == nil {
		return
	}
	tracker.mu.Lock()
	tracker.matched = true
	tracker.accountID = accountID
	tracker.status = statusCode
	tracker.mu.Unlock()
}

// GetOpsCustomAccountError returns the latest matched account custom error for
// the current request.
func GetOpsCustomAccountError(ctx context.Context) (accountID int64, statusCode int, matched bool) {
	if ctx == nil {
		return 0, 0, false
	}
	tracker, _ := ctx.Value(opsCustomAccountErrorTrackerKey{}).(*opsCustomAccountErrorTracker)
	if tracker == nil {
		return 0, 0, false
	}
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()
	return tracker.accountID, tracker.status, tracker.matched
}
