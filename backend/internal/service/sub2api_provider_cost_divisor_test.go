//go:build unit

package service

import (
	"math"
	"testing"
)

func TestNormalizeRemoteCostDivisor(t *testing.T) {
	defaultValue, err := normalizeRemoteCostDivisor(nil)
	if err != nil || defaultValue != 1 {
		t.Fatalf("nil divisor=(%v,%v), want (1,nil)", defaultValue, err)
	}

	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		value := value
		t.Run("reject", func(t *testing.T) {
			if _, err := normalizeRemoteCostDivisor(&value); err == nil {
				t.Fatalf("divisor %v should be rejected", value)
			}
		})
	}

	value := 10.0
	normalized, err := normalizeRemoteCostDivisor(&value)
	if err != nil || normalized != 10 {
		t.Fatalf("valid divisor=(%v,%v), want (10,nil)", normalized, err)
	}
}

func TestEffectiveRemoteCostDivisorDefaultsInvalidStoredValues(t *testing.T) {
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if got := effectiveRemoteCostDivisor(value); got != 1 {
			t.Fatalf("stored divisor %v normalized to %v, want 1", value, got)
		}
	}
	if got := effectiveRemoteCostDivisor(10); got != 10 {
		t.Fatalf("stored divisor 10 normalized to %v", got)
	}
}
