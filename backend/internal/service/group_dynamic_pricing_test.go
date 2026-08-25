package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupApplyDynamicPricingFloor(t *testing.T) {
	tests := []struct {
		name     string
		group    *Group
		userRate float64
		want     float64
	}{
		{
			name:     "dynamic override below floor uses group rate",
			group:    &Group{DynamicPricingEnabled: true, RateMultiplier: 0.10},
			userRate: 0.06,
			want:     0.10,
		},
		{
			name:     "dynamic override above floor is retained",
			group:    &Group{DynamicPricingEnabled: true, RateMultiplier: 0.10},
			userRate: 0.12,
			want:     0.12,
		},
		{
			name:     "static group preserves historical lower override",
			group:    &Group{DynamicPricingEnabled: false, RateMultiplier: 0.10},
			userRate: 0.06,
			want:     0.06,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.InDelta(t, tt.want, tt.group.ApplyDynamicPricingFloor(tt.userRate), 1e-12)
		})
	}
}

func TestValidateDynamicPricingConfig(t *testing.T) {
	require.NoError(t, validateDynamicPricingConfig(0.20, 0.02))
	require.Error(t, validateDynamicPricingConfig(0, 0.02))
	require.Error(t, validateDynamicPricingConfig(0.20, -0.01))
	require.Error(t, validateDynamicPricingConfig(math.NaN(), 0.02))
	require.Error(t, validateDynamicPricingConfig(0.20, math.Inf(1)))
}
