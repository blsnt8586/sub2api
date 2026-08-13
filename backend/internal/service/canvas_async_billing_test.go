//go:build unit

package service

import "testing"

func TestCanvasAsyncBillingRequestID(t *testing.T) {
	if got := CanvasAsyncBillingRequestID("  "); got != "" {
		t.Fatalf("blank task id: want empty, got %q", got)
	}
	if got := CanvasAsyncBillingRequestID(" task_abc "); got != "canvas_async_task:task_abc" {
		t.Fatalf("want canvas_async_task:task_abc, got %q", got)
	}
}

func TestCanvasAsyncTerminalSuccess(t *testing.T) {
	cases := map[string]bool{
		"succeeded":  true, // 图像/音频上游
		"completed":  true, // 视频上游
		"COMPLETED":  true, // 大小写不敏感
		"processing": false,
		"queued":     false,
		"pending":    false,
		"failed":     false,
		"cancelled":  false,
		"canceled":   false,
		"":           false,
	}
	for status, want := range cases {
		if got := canvasAsyncTerminalSuccess(status); got != want {
			t.Errorf("canvasAsyncTerminalSuccess(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestApplyCanvasAsyncCompletionBillingImage(t *testing.T) {
	result := &OpenAIForwardResult{Model: "req-model"}
	body := []byte(`{"status":"succeeded","model":"seedream-4.0","result":{"data":[{"url":"https://x/y.png","width":1368,"height":768}]}}`)

	billed := applyCanvasAsyncCompletionBilling(result, body, "task_img_1", "image")
	if !billed {
		t.Fatal("succeeded status must bill")
	}
	if result.CanvasImageCount != 1 {
		t.Errorf("CanvasImageCount = %d, want 1", result.CanvasImageCount)
	}
	if result.VideoCount != 0 {
		t.Errorf("VideoCount = %d, want 0 for image capability", result.VideoCount)
	}
	if result.ResponseID != "task_img_1" {
		t.Errorf("ResponseID = %q, want task_img_1", result.ResponseID)
	}
	// model 从响应体覆盖（轮询请求体为空，拿不到 requestModel）
	if result.Model != "seedream-4.0" || result.BillingModel != "seedream-4.0" {
		t.Errorf("model not taken from response body: Model=%q BillingModel=%q", result.Model, result.BillingModel)
	}
	// 图像不填 VideoSeconds
	if result.VideoSeconds != 0 {
		t.Errorf("image VideoSeconds = %d, want 0", result.VideoSeconds)
	}
	if len(result.ImageOutputSizes) != 1 || result.ImageOutputSizes[0] != "1368x768" {
		t.Errorf("ImageOutputSizes = %#v, want [1368x768]", result.ImageOutputSizes)
	}
	if result.ImageOutputSize != "1368x768" {
		t.Errorf("ImageOutputSize = %q, want 1368x768", result.ImageOutputSize)
	}
}

func TestApplyCanvasAsyncCompletionBillingImageUsesActualOutputCount(t *testing.T) {
	result := &OpenAIForwardResult{Model: "nano-banana-2"}
	body := []byte(`{"status":"succeeded","model":"nano-banana-2","result":{"data":[{"url":"https://x/1.png","width":1024,"height":1024},{"url":"https://x/2.png","width":2048,"height":2048}]}}`)

	if !applyCanvasAsyncCompletionBilling(result, body, "task_img_2", "image") {
		t.Fatal("succeeded status must bill")
	}
	if result.CanvasImageCount != 2 {
		t.Fatalf("CanvasImageCount = %d, want actual output count 2", result.CanvasImageCount)
	}
	if len(result.ImageOutputSizes) != 2 || result.ImageOutputSizes[1] != "2048x2048" {
		t.Fatalf("ImageOutputSizes = %#v", result.ImageOutputSizes)
	}
}

func TestApplyCanvasAsyncCompletionBillingImageWithNoOutputsDoesNotBill(t *testing.T) {
	result := &OpenAIForwardResult{Model: "nano-banana-2"}
	body := []byte(`{"status":"succeeded","model":"nano-banana-2","result":{"data":[]}}`)

	if applyCanvasAsyncCompletionBilling(result, body, "task_img_empty", "image") {
		t.Fatal("succeeded image task without outputs must not bill")
	}
	if result.CanvasImageCount != 0 {
		t.Fatalf("CanvasImageCount = %d, want 0", result.CanvasImageCount)
	}
}

func TestApplyCanvasAsyncCompletionBillingAudio(t *testing.T) {
	result := &OpenAIForwardResult{Model: "tts-model"}
	body := []byte(`{"status":"succeeded","model":"tts-model"}`)

	billed := applyCanvasAsyncCompletionBilling(result, body, "task_aud_1", "audio")
	if !billed {
		t.Fatal("succeeded status must bill")
	}
	if result.CanvasAudioCount != 1 {
		t.Errorf("CanvasAudioCount = %d, want 1", result.CanvasAudioCount)
	}
	if result.VideoCount != 0 {
		t.Errorf("VideoCount = %d, want 0 for audio capability", result.VideoCount)
	}
}

func TestApplyCanvasAsyncCompletionBillingVideoSeconds(t *testing.T) {
	result := &OpenAIForwardResult{Model: "veo-3.1"}
	body := []byte(`{"status":"completed","model":"veo-3.1","result":{"data":[{"url":"https://x/y.mp4","duration":8}]}}`)

	billed := applyCanvasAsyncCompletionBilling(result, body, "task_vid_1", "video")
	if !billed {
		t.Fatal("completed status must bill")
	}
	if result.VideoCount != 1 {
		t.Errorf("VideoCount = %d, want 1", result.VideoCount)
	}
	if result.VideoSeconds != 8 {
		t.Errorf("VideoSeconds = %d, want 8 (from result.data[0].duration)", result.VideoSeconds)
	}
}

func TestApplyCanvasAsyncCompletionBillingNonTerminalOrFailed(t *testing.T) {
	for _, status := range []string{"processing", "queued", "pending", "failed", "cancelled"} {
		result := &OpenAIForwardResult{Model: "m"}
		body := []byte(`{"status":"` + status + `","model":"m"}`)
		if applyCanvasAsyncCompletionBilling(result, body, "task_x", "image") {
			t.Errorf("status %q must NOT bill", status)
		}
		if result.VideoCount != 0 || result.CanvasImageCount != 0 || result.ResponseID != "" {
			t.Errorf("status %q must not fill billing fields: VideoCount=%d CanvasImageCount=%d ResponseID=%q",
				status, result.VideoCount, result.CanvasImageCount, result.ResponseID)
		}
	}
}

func TestApplyCanvasAsyncCompletionBillingNilAndInvalid(t *testing.T) {
	if applyCanvasAsyncCompletionBilling(nil, []byte(`{"status":"succeeded"}`), "t", "image") {
		t.Error("nil result must return false")
	}
	result := &OpenAIForwardResult{Model: "m"}
	if applyCanvasAsyncCompletionBilling(result, []byte(`not-json`), "t", "image") {
		t.Error("invalid json must return false (no status)")
	}
}
