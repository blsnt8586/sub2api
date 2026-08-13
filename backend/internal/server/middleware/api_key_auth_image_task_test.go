package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAsyncImageTaskRead(t *testing.T) {
	require.True(t, isAsyncImageTaskRead(http.MethodGet, "/v1/images/tasks/imgtask_123"))
	require.True(t, isAsyncImageTaskRead(http.MethodGet, "/images/tasks/imgtask_123"))
	require.True(t, isAsyncImageTaskRead(http.MethodGet, "/v1/images/imgtask_123"))
	require.True(t, isAsyncImageTaskRead(http.MethodGet, "/images/imgtask_123"))
	require.True(t, isAsyncImageTaskRead(http.MethodGet, "/v1/tasks/imgtask_123"))
	require.True(t, isAsyncImageTaskRead(http.MethodGet, "/tasks/imgtask_123"))
	require.False(t, isAsyncImageTaskRead(http.MethodPost, "/v1/images/tasks/imgtask_123"))
	require.False(t, isAsyncImageTaskRead(http.MethodGet, "/v1/images/generations"))
	require.False(t, isAsyncImageTaskRead(http.MethodGet, "/v1/images/batches"))
	require.False(t, isAsyncImageTaskRead(http.MethodGet, "/v1/images/batches/batch_123"))
	require.False(t, isAsyncImageTaskRead(http.MethodGet, "/v1/images/"))
	require.False(t, isAsyncImageTaskRead(http.MethodGet, "/v1/tasks/"))
}
