//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardCanvasAsyncImageJSONPreservesSelectedSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"gpt-image-2","prompt":"landscape","size":"1376x768","n":1}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", "image-json-123")

	upstream := &httpUpstreamRecorder{resp: asyncImageTaskResponse(`{"id":"img_landscape","status":"queued"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardCanvasAsyncImage(
		context.Background(), c, newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret"),
		CanvasAsyncImageCreate, "", body, "application/json",
	)
	require.NoError(t, err)
	require.Equal(t, "https://zz1cc.cc.cd/v1/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, body, upstream.lastBody)
	require.Equal(t, "1376x768", gjson.GetBytes(upstream.lastBody, "size").String())
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	require.Equal(t, "image-json-123", upstream.lastReq.Header.Get("Idempotency-Key"))
	require.Equal(t, "img_landscape", result.ResponseID)
}

func TestForwardCanvasAsyncImageMultipartPreservesSelectedSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("model", "gpt-image-2"))
	require.NoError(t, mw.WriteField("prompt", "portrait edit"))
	require.NoError(t, mw.WriteField("size", "896x1200"))
	image, err := mw.CreateFormFile("image[]", "reference.png")
	require.NoError(t, err)
	_, err = image.Write([]byte("REFERENCE_IMAGE_BYTES"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	body := buf.Bytes()
	contentType := mw.FormDataContentType()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.Header.Set("Idempotency-Key", "image-multipart-123")

	upstream := &httpUpstreamRecorder{resp: asyncImageTaskResponse(`{"id":"img_portrait","status":"queued"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardCanvasAsyncImage(
		context.Background(), c, newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret"),
		CanvasAsyncImageCreate, "", body, contentType,
	)
	require.NoError(t, err)
	require.Equal(t, contentType, upstream.lastReq.Header.Get("Content-Type"))
	require.Equal(t, "image-multipart-123", upstream.lastReq.Header.Get("Idempotency-Key"))
	require.Equal(t, body, upstream.lastBody)

	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	form, err := multipart.NewReader(bytes.NewReader(upstream.lastBody), params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	require.Equal(t, []string{"896x1200"}, form.Value["size"])
	require.Equal(t, "img_portrait", result.ResponseID)
}

func TestForwardCanvasAsyncImagePreservesStructuredClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"gpt-image-2","prompt":"landscape","size":"999x999","n":1}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"unsupported image size","type":"invalid_request","code":"invalid_size"}}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	_, err := svc.ForwardCanvasAsyncImage(
		context.Background(), c, newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret"),
		CanvasAsyncImageCreate, "", body, "application/json",
	)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{"error":{"message":"unsupported image size","type":"invalid_request","code":"invalid_size"}}`, recorder.Body.String())
}

func TestCanvasAsyncImageModerationBodyJSONAndMultipart(t *testing.T) {
	jsonBody := CanvasAsyncImageModerationBody("application/json", []byte(`{"model":"gpt-image-2","prompt":"  audit this prompt  "}`))
	require.JSONEq(t, `{"prompt":"audit this prompt"}`, string(jsonBody))

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("model", "nano-banana-pro"))
	require.NoError(t, mw.WriteField("prompt", "  audit multipart prompt  "))
	image, err := mw.CreateFormFile("image[]", "reference.png")
	require.NoError(t, err)
	_, err = image.Write([]byte("REFERENCE_IMAGE_BYTES"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	multipartBody := CanvasAsyncImageModerationBody(mw.FormDataContentType(), buf.Bytes())
	require.JSONEq(t, `{"prompt":"audit multipart prompt"}`, string(multipartBody))
	require.NotContains(t, string(multipartBody), "REFERENCE_IMAGE_BYTES")
}

func TestForwardCanvasAsyncImageRateLimitTriggersFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"gpt-image-2","prompt":"landscape","size":"1376x768","n":1}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"account queue full","code":"account_queue_full"}}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	_, err := svc.ForwardCanvasAsyncImage(
		context.Background(), c, newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret"),
		CanvasAsyncImageCreate, "", body, "application/json",
	)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Empty(t, recorder.Body.String(), "failover response must not be committed before another account is tried")
}

func asyncImageTaskResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusAccepted,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
