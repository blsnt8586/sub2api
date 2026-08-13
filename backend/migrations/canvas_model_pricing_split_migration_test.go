//go:build unit

package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanvasModelPricingSplitMigrationPreservesBothJSONShapes(t *testing.T) {
	sqlBytes, err := FS.ReadFile("222_canvas_model_pricing_split.sql")
	require.NoError(t, err)
	sqlText := strings.ToLower(string(sqlBytes))

	require.Contains(t, sqlText, "add column if not exists canvas_model_pricing jsonb")
	require.Contains(t, sqlText, "jsonb_typeof(model_pricing) = 'object'")
	require.Contains(t, sqlText, "canvas_model_pricing = coalesce(canvas_model_pricing, model_pricing)")
	require.Contains(t, sqlText, "model_pricing = null")
	require.NotContains(t, sqlText, "jsonb_typeof(model_pricing) = 'array'")
}
