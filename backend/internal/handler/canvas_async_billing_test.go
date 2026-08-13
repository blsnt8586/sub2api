//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestShouldRecordCanvasAsyncCompletionUsage(t *testing.T) {
	tests := []struct {
		name       string
		result     *service.OpenAIForwardResult
		capability string
		want       bool
	}{
		{
			name:       "image accepts independent image marker",
			result:     &service.OpenAIForwardResult{CanvasImageCount: 1},
			capability: "image",
			want:       true,
		},
		{
			name:       "image rejects video marker",
			result:     &service.OpenAIForwardResult{VideoCount: 1},
			capability: "image",
			want:       false,
		},
		{
			name:       "audio accepts independent audio marker",
			result:     &service.OpenAIForwardResult{CanvasAudioCount: 1},
			capability: "audio",
			want:       true,
		},
		{
			name:       "video accepts video marker",
			result:     &service.OpenAIForwardResult{VideoCount: 1},
			capability: "video",
			want:       true,
		},
		{
			name:       "nil result",
			result:     nil,
			capability: "image",
			want:       false,
		},
		{
			name:       "unknown capability",
			result:     &service.OpenAIForwardResult{CanvasImageCount: 1, CanvasAudioCount: 1, VideoCount: 1},
			capability: "document",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shouldRecordCanvasAsyncCompletionUsage(tt.result, tt.capability))
		})
	}
}
