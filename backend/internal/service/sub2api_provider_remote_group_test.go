//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
	"github.com/stretchr/testify/require"
)

func TestResolveProviderRemoteKeyGroupPrefersEmbeddedGroup(t *testing.T) {
	key := sub2api.APIKey{GroupID: 7}
	key.Group = &struct {
		ID             int64   `json:"id"`
		Name           string  `json:"name"`
		RateMultiplier float64 `json:"rate_multiplier"`
	}{ID: 9, Name: "embedded", RateMultiplier: 0.2}

	group, ok := resolveProviderRemoteKeyGroup(key, map[int64]sub2api.Group{
		7: {ID: 7, Name: "lookup", RateMultiplier: 0.1},
	})
	require.True(t, ok)
	require.Equal(t, providerRemoteGroupInfo{id: 9, name: "embedded", multiplier: 0.2, complete: true}, group)
}

func TestResolveProviderRemoteKeyGroupCompletesGroupIDFromCatalog(t *testing.T) {
	group, ok := resolveProviderRemoteKeyGroup(sub2api.APIKey{GroupID: 7}, map[int64]sub2api.Group{
		7: {ID: 7, Name: "catalog", RateMultiplier: 0.08},
	})
	require.True(t, ok)
	require.Equal(t, providerRemoteGroupInfo{id: 7, name: "catalog", multiplier: 0.08, complete: true}, group)
}

func TestResolveProviderRemoteKeyGroupKeepsIdentityWhenCatalogUnavailable(t *testing.T) {
	group, ok := resolveProviderRemoteKeyGroup(sub2api.APIKey{GroupID: 7}, nil)
	require.True(t, ok)
	require.Equal(t, providerRemoteGroupInfo{id: 7}, group)

	_, ok = resolveProviderRemoteKeyGroup(sub2api.APIKey{}, nil)
	require.False(t, ok)
}
