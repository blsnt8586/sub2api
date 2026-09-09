package service

import (
	"context"
	"fmt"
	"time"
)

const (
	// SmartGroupSwitchLogRetention is the user-visible and database retention
	// window for automatic API-key route changes.
	SmartGroupSwitchLogRetention = 3 * 24 * time.Hour
	SmartGroupSwitchLogMaxItems  = 100
)

// ListSmartGroupSwitchLogs returns only logs belonging to the specified user
// and API key.  The repository implementation applies the ownership check in
// SQL as a second line of defence; callers should still perform their normal
// API-key ownership check before invoking it.
func (s *APIKeyService) ListSmartGroupSwitchLogs(ctx context.Context, userID, apiKeyID int64) ([]APIKeySmartGroupSwitchLog, error) {
	if s == nil || s.apiKeyRepo == nil {
		return nil, fmt.Errorf("API key service is unavailable")
	}
	repo, ok := s.apiKeyRepo.(APIKeySmartGroupLogRepository)
	if !ok {
		return nil, fmt.Errorf("smart group switch log repository is unavailable")
	}
	return repo.ListSmartGroupSwitchLogs(ctx, userID, apiKeyID, time.Now().Add(-SmartGroupSwitchLogRetention), SmartGroupSwitchLogMaxItems)
}
