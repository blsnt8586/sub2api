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

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIVideoUpstreamStub struct {
	response *http.Response
	request  *http.Request
	body     []byte
}

func (s *openAIVideoUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.request = req
	if req.Body != nil {
		s.body, _ = io.ReadAll(req.Body)
	}
	return s.response, nil
}

func (s *openAIVideoUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, accountConcurrency)
}

func openAIVideoTestService(upstream *openAIVideoUpstreamStub) *OpenAIGatewayService {
	return &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
			Enabled: false,
		}}},
	}
}

func openAIVideoTestAccount() *Account {
	return &Account{
		ID: 7, Name: "openai-video", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-upstream", "base_url": "https://upstream.test/v1",
			"model_mapping": map[string]any{"video-public": "sora-2-pro"},
		},
	}
}

func openAIVideoGinContext(method, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

func TestForwardOpenAIVideoCreateUsesOfficialProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &openAIVideoUpstreamStub{response: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(bytes.NewBufferString(`{"id":"video_123","status":"queued"}`)),
	}}
	svc := openAIVideoTestService(upstream)
	body := []byte(`{"model":"video-public","prompt":"waves","duration":20,"resolution":"1080p","size":"1920x1080","fps":24}`)
	c, recorder := openAIVideoGinContext(http.MethodPost, "/v1/videos", body)

	result, err := svc.ForwardOpenAIVideo(context.Background(), c, openAIVideoTestAccount(), OpenAIVideoEndpointCreate, "", body, "application/json", "")

	require.NoError(t, err)
	require.Equal(t, "https://upstream.test/v1/videos", upstream.request.URL.String())
	require.Equal(t, "Bearer sk-upstream", upstream.request.Header.Get("Authorization"))
	require.Equal(t, "sora-2-pro", gjson.GetBytes(upstream.body, "model").String())
	require.Equal(t, int64(20), gjson.GetBytes(upstream.body, "seconds").Int())
	require.False(t, gjson.GetBytes(upstream.body, "duration").Exists())
	require.False(t, gjson.GetBytes(upstream.body, "resolution").Exists())
	require.False(t, gjson.GetBytes(upstream.body, "fps").Exists())
	require.Equal(t, "video_123", result.ResponseID)
	require.Equal(t, VideoBillingResolution1080P, result.VideoResolution)
	require.Equal(t, 20, result.VideoDurationSeconds)
	require.Equal(t, 0, recorder.Body.Len(), "create response must remain buffered until task state is persisted")
	svc.CommitOpenAIVideoResponse(c, result)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"id":"video_123","status":"queued"}`, recorder.Body.String())
}

func TestForwardOpenAIVideoCreateNormalizesMultipartProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	require.NoError(t, writer.WriteField("model", "video-public"))
	require.NoError(t, writer.WriteField("prompt", "waves"))
	require.NoError(t, writer.WriteField("duration", "20"))
	require.NoError(t, writer.WriteField("resolution", "1080p"))
	require.NoError(t, writer.WriteField("fps", "24"))
	require.NoError(t, writer.WriteField("generate_audio", "true"))
	file, err := writer.CreateFormFile("input_reference", "reference.png")
	require.NoError(t, err)
	_, err = file.Write([]byte("png!"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	upstream := &openAIVideoUpstreamStub{response: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(bytes.NewBufferString(`{"id":"video_multipart","status":"queued"}`)),
	}}
	c, _ := openAIVideoGinContext(http.MethodPost, "/v1/videos/generations", requestBody.Bytes())
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	result, err := openAIVideoTestService(upstream).ForwardOpenAIVideo(
		context.Background(), c, openAIVideoTestAccount(), OpenAIVideoEndpointCreate,
		"", requestBody.Bytes(), writer.FormDataContentType(), "",
	)
	require.NoError(t, err)
	require.Equal(t, "video_multipart", result.ResponseID)

	mediaType, params, err := mime.ParseMediaType(upstream.request.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	reader := multipart.NewReader(bytes.NewReader(upstream.body), params["boundary"])
	fields := map[string]string{}
	fileBody := ""
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		require.NoError(t, nextErr)
		value, readErr := io.ReadAll(part)
		require.NoError(t, readErr)
		if part.FileName() != "" {
			fileBody = string(value)
		} else {
			fields[part.FormName()] = string(value)
		}
	}
	require.Equal(t, "sora-2-pro", fields["model"])
	require.Equal(t, "20", fields["seconds"])
	require.Equal(t, "waves", fields["prompt"])
	require.NotContains(t, fields, "duration")
	require.NotContains(t, fields, "resolution")
	require.NotContains(t, fields, "fps")
	require.NotContains(t, fields, "generate_audio")
	require.Equal(t, "png!", fileBody)
}

func TestForwardOpenAIVideoStatusMarksCompletedForBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &openAIVideoUpstreamStub{response: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(bytes.NewBufferString(`{"id":"video_123","status":"completed","model":"sora-2-pro","size":"1792x1024","seconds":"20"}`)),
	}}
	c, _ := openAIVideoGinContext(http.MethodGet, "/v1/videos/video_123", nil)

	result, err := openAIVideoTestService(upstream).ForwardOpenAIVideo(context.Background(), c, openAIVideoTestAccount(), OpenAIVideoEndpointStatus, "video_123", nil, "", "")

	require.NoError(t, err)
	require.Equal(t, "https://upstream.test/v1/videos/video_123", upstream.request.URL.String())
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, VideoBillingResolution1024P, result.VideoResolution)
	require.Equal(t, 20, result.VideoDurationSeconds)
}

func TestForwardOpenAIVideoContentProxiesBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &openAIVideoUpstreamStub{response: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"video/mp4"}, "Content-Length": []string{"4"}},
		Body: io.NopCloser(bytes.NewReader([]byte("mp4!"))),
	}}
	c, recorder := openAIVideoGinContext(http.MethodGet, "/v1/videos/video_123/content", nil)
	c.Request.Header.Set("Range", "bytes=0-3")

	result, err := openAIVideoTestService(upstream).ForwardOpenAIVideo(context.Background(), c, openAIVideoTestAccount(), OpenAIVideoEndpointContent, "video_123", nil, "", "")

	require.NoError(t, err)
	require.Equal(t, "https://upstream.test/v1/videos/video_123/content", upstream.request.URL.String())
	require.Equal(t, "bytes=0-3", upstream.request.Header.Get("Range"))
	require.Equal(t, "mp4!", recorder.Body.String())
	require.Equal(t, 1, result.VideoCount)
}

func TestForwardOpenAIVideoNonRetryableErrorPreservesUpstreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &openAIVideoUpstreamStub{response: &http.Response{
		StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"error":{"message":"invalid video size"}}`)),
	}}
	body := []byte(`{"model":"video-public","prompt":"waves","seconds":"8","size":"1280x720"}`)
	c, recorder := openAIVideoGinContext(http.MethodPost, "/v1/videos", body)

	result, err := openAIVideoTestService(upstream).ForwardOpenAIVideo(
		context.Background(), c, openAIVideoTestAccount(), OpenAIVideoEndpointCreate,
		"", body, "application/json", "",
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{"error":{"message":"invalid video size"}}`, recorder.Body.String())
}

func TestOpenAIVideoCapabilityRequiresAPIKeyAccount(t *testing.T) {
	require.True(t, openAIVideoTestAccount().SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos))
	require.False(t, (&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}).SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos))
	require.False(t, (&Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}).SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos))
}

func TestSelectBoundOpenAIVideoAccountUsesOnlyCreatingAccount(t *testing.T) {
	creating := Account{
		ID: 7, Name: "creating", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
	}
	other := Account{
		ID: 8, Name: "other", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
	}
	svc := &OpenAIGatewayService{accountRepo: stubOpenAIAccountRepo{accounts: []Account{other, creating}}}

	selection, err := svc.SelectBoundOpenAIVideoAccount(context.Background(), nil, creating.ID)

	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, creating.ID, selection.Account.ID)
	require.True(t, selection.Acquired)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}
