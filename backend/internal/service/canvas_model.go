package service

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const canvasModelFieldMaxSize = 1 << 20

// CanvasUpstreamModel returns the model slug used by the local AVI2API
// capability registry. Canvas keeps the provider prefix (for example,
// "leonardo/gpt-image-2") for routing and billing; this helper is used only
// when consulting the registry for validation. The actual outbound model is
// selected by CanvasMappedUpstreamModel and is not stripped implicitly.
func CanvasUpstreamModel(model string) string {
	model = strings.TrimSpace(model)
	if idx := strings.LastIndexByte(model, '/'); idx >= 0 {
		return strings.TrimSpace(model[idx+1:])
	}
	return model
}

// CanvasMappedUpstreamModel resolves an account's model_mapping for the
// upstream request.  The mapped value is intentionally kept exactly as
// configured: current AVI2API catalogs use the complete provider/model ID
// (for example, "leonardo/gpt-image-2") to select the provider.  An account
// that targets a legacy bare-model AVI2API can explicitly configure a bare
// mapping value, which is then sent unchanged.
func CanvasMappedUpstreamModel(account *Account, requestedModel string) string {
	requestedModel = strings.TrimSpace(requestedModel)
	mappedModel := requestedModel
	if account != nil {
		if candidate, matched := account.ResolveMappedModel(requestedModel); matched && strings.TrimSpace(candidate) != "" {
			mappedModel = candidate
		}
	}
	return strings.TrimSpace(mappedModel)
}

// RewriteCanvasModel rewrites only the top-level model field in a Canvas
// request body.  JSON is edited in-place with sjson; multipart bodies are
// rebuilt with the original boundary while copying every part (including
// binary reference files) byte-for-byte.  When the requested value already
// equals the replacement, the original slice is returned unchanged.
func RewriteCanvasModel(contentType string, body []byte, model string) ([]byte, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return body, nil
	}
	current := extractCanvasBodyModel(contentType, body)
	if current == model {
		return body, nil
	}

	if gjson.ValidBytes(body) {
		if !gjson.GetBytes(body, "model").Exists() {
			return body, nil
		}
		rewritten, err := sjson.SetBytes(body, "model", model)
		if err != nil {
			return nil, fmt.Errorf("rewrite Canvas JSON model: %w", err)
		}
		return rewritten, nil
	}

	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return body, nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, fmt.Errorf("rewrite Canvas multipart model: boundary is missing")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var out bytes.Buffer
	writer := multipart.NewWriter(&out)
	if err := writer.SetBoundary(boundary); err != nil {
		return nil, fmt.Errorf("rewrite Canvas multipart model: invalid boundary: %w", err)
	}
	found := false
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, fmt.Errorf("rewrite Canvas multipart model: read part: %w", nextErr)
		}

		partWriter, createErr := writer.CreatePart(part.Header)
		if createErr != nil {
			_ = part.Close()
			return nil, fmt.Errorf("rewrite Canvas multipart model: create part: %w", createErr)
		}
		if !found && part.FormName() == "model" && strings.TrimSpace(part.FileName()) == "" {
			if _, writeErr := partWriter.Write([]byte(model)); writeErr != nil {
				_ = part.Close()
				return nil, fmt.Errorf("rewrite Canvas multipart model: write model part: %w", writeErr)
			}
			found = true
		} else if _, copyErr := io.Copy(partWriter, part); copyErr != nil {
			_ = part.Close()
			return nil, fmt.Errorf("rewrite Canvas multipart model: copy part: %w", copyErr)
		}
		_ = part.Close()
	}
	if !found {
		return body, nil
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("rewrite Canvas multipart model: close writer: %w", err)
	}
	return out.Bytes(), nil
}

func extractCanvasBodyModel(contentType string, body []byte) string {
	if gjson.ValidBytes(body) {
		return strings.TrimSpace(gjson.GetBytes(body, "model").String())
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return ""
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return ""
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, nextErr := reader.NextPart()
		if nextErr != nil {
			return ""
		}
		name := part.FormName()
		filename := part.FileName()
		if name == "model" && strings.TrimSpace(filename) == "" {
			data, readErr := io.ReadAll(io.LimitReader(part, canvasModelFieldMaxSize))
			_ = part.Close()
			if readErr != nil {
				return ""
			}
			return strings.TrimSpace(string(data))
		}
		// Drain file and unrelated form parts without allocating a second copy
		// of potentially hundreds of megabytes of reference media.
		_, _ = io.Copy(io.Discard, part)
		_ = part.Close()
	}
}

// CanvasValidationBody strips only the provider prefix for local AVI2API
// schema validation.  The original body remains the body used for routing,
// audit, logging, and billing; this helper is validation-only.
func CanvasValidationBody(contentType string, body []byte) ([]byte, error) {
	model := extractCanvasBodyModel(contentType, body)
	if model == "" {
		return body, nil
	}
	return RewriteCanvasModel(contentType, body, CanvasUpstreamModel(model))
}
