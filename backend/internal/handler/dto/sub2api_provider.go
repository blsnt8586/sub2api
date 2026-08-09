package dto

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Provider DTO
type Provider struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	BaseURL        string  `json:"base_url"`
	ProviderType   string  `json:"provider_type"`
	Status         string  `json:"status"`
	Notes          *string `json:"notes,omitempty"`
	Email          string  `json:"email"`
	APIPathKeys    *string `json:"api_path_keys,omitempty"`
	APIPathGroups  *string `json:"api_path_groups,omitempty"`
	LastSyncAt     *string `json:"last_sync_at,omitempty"`
	LastSyncStatus *string `json:"last_sync_status,omitempty"`
	LastSyncError  *string `json:"last_sync_error,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	AccountsCount  int     `json:"accounts_count"`
}

// ProviderFromService 从 Service 层转换为 DTO
func ProviderFromService(s *service.Provider) *Provider {
	if s == nil {
		return nil
	}

	return &Provider{
		ID:             s.ID,
		Name:           s.Name,
		BaseURL:        s.BaseURL,
		ProviderType:   s.ProviderType,
		Status:         s.Status,
		Notes:          s.Notes,
		Email:          s.Email,
		APIPathKeys:    s.APIPathKeys,
		APIPathGroups:  s.APIPathGroups,
		LastSyncAt:     s.LastSyncAt,
		LastSyncStatus: s.LastSyncStatus,
		LastSyncError:  s.LastSyncError,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
		AccountsCount:  s.AccountsCount,
	}
}
