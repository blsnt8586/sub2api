package admin

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func canvasAccount(id int64, mapping map[string]any) service.Account {
	return service.Account{
		ID:          id,
		Platform:    service.PlatformCanvas,
		Status:      service.StatusActive,
		Credentials: map[string]any{"model_mapping": mapping},
	}
}

func TestCollectCanvasPricingModelsKeepsPlatformQualifiedIDs(t *testing.T) {
	video, image, audio := collectCanvasPricingModels([]service.Account{
		canvasAccount(1, map[string]any{
			"leonardo/gpt-image-2": "leonardo/gpt-image-2",
			"adobe/gpt-image-2":    "adobe/gpt-image-2",
			"leonardo/kling-3":     "leonardo/kling-3",
			"leonardo/music-v1":    "leonardo/music-v1",
		}),
		canvasAccount(2, map[string]any{
			"adobe/gpt-image-2":       "adobe/gpt-image-2",
			"adobe/Gemini Omni Flash": "adobe/Gemini Omni Flash",
			"unknown/text-model":      "unknown/text-model",
		}),
	})

	require.Equal(t, []string{"adobe/Gemini Omni Flash", "leonardo/kling-3"}, video)
	require.Equal(t, []string{"adobe/gpt-image-2", "leonardo/gpt-image-2"}, image)
	require.Equal(t, []string{"leonardo/music-v1"}, audio)
}

func TestGetCanvasPricingModelsReadsActiveCanvasAccounts(t *testing.T) {
	stub := newStubAdminService()
	stub.accountSchedulerScoreFilterAccounts = []service.Account{
		canvasAccount(7, map[string]any{"leonardo/gpt-image-2": "leonardo/gpt-image-2"}),
		{Platform: service.PlatformCanvas, Status: service.StatusError, Credentials: map[string]any{"model_mapping": map[string]any{"adobe/gpt-image-2": "adobe/gpt-image-2"}}},
		{Platform: service.PlatformOpenAI, Status: service.StatusActive, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5": "gpt-5"}}},
	}
	h := NewGroupHandler(stub, nil, nil)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/groups/canvas-pricing-models", nil)
	h.GetCanvasPricingModels(c)

	require.Equal(t, 200, recorder.Code)
	require.Equal(t, 1, stub.schedulerScoreFilterCalls)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	data, ok := envelope["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{"leonardo/gpt-image-2"}, data["image"])
}

func TestCollectCanvasPricingModelsReturnsEmptyWhenNoAccounts(t *testing.T) {
	video, image, audio := collectCanvasPricingModels(nil)
	require.Empty(t, video)
	require.Empty(t, image)
	require.Empty(t, audio)
}
