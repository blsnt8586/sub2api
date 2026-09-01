//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCanvasUpstreamModelStripsOnlyProviderPrefix(t *testing.T) {
	cases := map[string]string{
		"leonardo/gpt-image-2":   "gpt-image-2",
		"leonardo/kling-o3-omni": "kling-o3-omni",
		"gpt-image-2":            "gpt-image-2",
		"  provider/model/name ": "name",
		"provider/ ":             "",
	}
	for input, want := range cases {
		require.Equal(t, want, CanvasUpstreamModel(input), input)
	}
}

func TestCanvasMappedUpstreamModelPreservesConfiguredProviderModel(t *testing.T) {
	account := newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret")
	account.Credentials["model_mapping"] = map[string]any{
		"leonardo/gpt-image-2": "leonardo/nano-banana-pro",
	}
	require.Equal(t, "leonardo/nano-banana-pro", CanvasMappedUpstreamModel(account, "leonardo/gpt-image-2"))
	// A mapping for another provider must not be selected for this request.
	require.Equal(t, "other/gpt-image-2", CanvasMappedUpstreamModel(account, "other/gpt-image-2"))
	account.Credentials["model_mapping"] = map[string]any{"leonardo/gpt-image-2": "gpt-image-2"}
	require.Equal(t, "gpt-image-2", CanvasMappedUpstreamModel(account, "leonardo/gpt-image-2"))
}

func TestRewriteCanvasModelJSON(t *testing.T) {
	body := []byte(`{"model":"leonardo/gpt-image-2","prompt":"cat","n":1}`)
	rewritten, err := RewriteCanvasModel("application/json", body, "gpt-image-2")
	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", gjson.GetBytes(rewritten, "model").String())
	require.Equal(t, "cat", gjson.GetBytes(rewritten, "prompt").String())
	require.Equal(t, int64(1), gjson.GetBytes(rewritten, "n").Int())

	unchanged, err := RewriteCanvasModel("application/json", body, "leonardo/gpt-image-2")
	require.NoError(t, err)
	require.Equal(t, body, unchanged)
}

func TestRewriteCanvasModelMultipartPreservesReferenceBytes(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("model", "leonardo/gpt-image-2"))
	require.NoError(t, mw.WriteField("prompt", "edit"))
	ref, err := mw.CreateFormFile("image[]", "ref.png")
	require.NoError(t, err)
	const reference = "PNG_REFERENCE_BYTES"
	_, err = ref.Write([]byte(reference))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	rewritten, err := RewriteCanvasModel(mw.FormDataContentType(), buf.Bytes(), "gpt-image-2")
	require.NoError(t, err)
	form, err := multipart.NewReader(bytes.NewReader(rewritten), strings.TrimPrefix(mw.FormDataContentType(), "multipart/form-data; boundary=")).ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	require.Equal(t, []string{"gpt-image-2"}, form.Value["model"])
	file, err := form.File["image[]"][0].Open()
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, reference, string(data))
}

func TestCanvasValidationAcceptsProviderPrefixedModels(t *testing.T) {
	require.Nil(t, ValidateCanvasAsyncImageRequest("application/json", []byte(`{"model":"leonardo/gpt-image-2","prompt":"cat"}`)))
	require.Nil(t, ValidateCanvasVideoRequest("application/json", []byte(`{"model":"leonardo/veo-3.1","prompt":"cat"}`)))
	require.Nil(t, ValidateCanvasAudioRequest([]byte(`{"model":"leonardo/sound-effects-v2","prompt":"thunder"}`)))
	// The live Canvas catalog may contain a model added after this binary's
	// optional capability registry; it must not be rejected before AVI2API sees it.
	require.Nil(t, ValidateCanvasVideoRequest("application/json", []byte(`{"model":"leonardo/happy-horse-1.1","prompt":"cat"}`)))
}

func TestForwardCanvasAsyncImagePreservesProviderModelForAVI2API(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"leonardo/gpt-image-2","prompt":"landscape","size":"1376x768","n":1}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: asyncImageTaskResponse(`{"id":"img-prefixed","status":"queued"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret")
	result, err := svc.ForwardCanvasAsyncImage(context.Background(), c, account, CanvasAsyncImageCreate, "", body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "leonardo/gpt-image-2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "leonardo/gpt-image-2", result.Model)
	require.Equal(t, "leonardo/gpt-image-2", result.BillingModel)
	require.Equal(t, "leonardo/gpt-image-2", result.UpstreamModel)
}

func TestForwardCanvasVideoPreservesProviderModelForAVI2API(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"leonardo/kling-o3-omni","prompt":"landscape"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusAccepted,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"video-prefixed","status":"queued"}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	result, err := svc.ForwardCanvasVideo(context.Background(), c, newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret"), CanvasVideoEndpointCreate, "", body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "leonardo/kling-o3-omni", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "leonardo/kling-o3-omni", result.Model)
	require.Equal(t, "leonardo/kling-o3-omni", result.UpstreamModel)
}

func TestForwardCanvasAudioPreservesProviderModelForAVI2API(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"leonardo/sound-effects-v2","prompt":"thunder"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusAccepted,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"audio-prefixed","status":"queued"}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	result, err := svc.ForwardCanvasAudio(context.Background(), c, newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret"), JimengAudioGeneration, "", body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "leonardo/sound-effects-v2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "leonardo/sound-effects-v2", result.Model)
	require.Equal(t, "leonardo/sound-effects-v2", result.UpstreamModel)
}
