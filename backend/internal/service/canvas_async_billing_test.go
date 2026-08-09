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

func TestApplyCanvasAsyncCompletionBillingSucceeded(t *testing.T) {
	result := &OpenAIForwardResult{Model: "req-model"}
	body := []byte(`{"status":"succeeded","model":"seedream-4.0","result":{"data":[{"url":"https://x/y.png"}]}}`)

	billed := applyCanvasAsyncCompletionBilling(result, body, "task_img_1", false)
	if !billed {
		t.Fatal("succeeded status must bill")
	}
	if result.VideoCount != 1 {
		t.Errorf("VideoCount = %d, want 1", result.VideoCount)
	}
	if result.ResponseID != "task_img_1" {
		t.Errorf("ResponseID = %q, want task_img_1", result.ResponseID)
	}
	// model 从响应体覆盖（轮询请求体为空，拿不到 requestModel）
	if result.Model != "seedream-4.0" || result.BillingModel != "seedream-4.0" {
		t.Errorf("model not taken from response body: Model=%q BillingModel=%q", result.Model, result.BillingModel)
	}
	// 图像 withSeconds=false，不应填 VideoSeconds
	if result.VideoSeconds != 0 {
		t.Errorf("image VideoSeconds = %d, want 0", result.VideoSeconds)
	}
}

func TestApplyCanvasAsyncCompletionBillingVideoSeconds(t *testing.T) {
	result := &OpenAIForwardResult{Model: "veo-3.1"}
	body := []byte(`{"status":"completed","model":"veo-3.1","result":{"data":[{"url":"https://x/y.mp4","duration":8}]}}`)

	billed := applyCanvasAsyncCompletionBilling(result, body, "task_vid_1", true)
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
		if applyCanvasAsyncCompletionBilling(result, body, "task_x", false) {
			t.Errorf("status %q must NOT bill", status)
		}
		if result.VideoCount != 0 || result.ResponseID != "" {
			t.Errorf("status %q must not fill billing fields: VideoCount=%d ResponseID=%q", status, result.VideoCount, result.ResponseID)
		}
	}
}

func TestApplyCanvasAsyncCompletionBillingNilAndInvalid(t *testing.T) {
	if applyCanvasAsyncCompletionBilling(nil, []byte(`{"status":"succeeded"}`), "t", false) {
		t.Error("nil result must return false")
	}
	result := &OpenAIForwardResult{Model: "m"}
	if applyCanvasAsyncCompletionBilling(result, []byte(`not-json`), "t", false) {
		t.Error("invalid json must return false (no status)")
	}
}
