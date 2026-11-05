package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

// Leading text whitespace remains in the attempt buffer until real output
// arrives. Do not trim the output itself: indentation and newlines must be
// preserved on success. Refusals, reasoning, tools and unknown event shapes
// retain their existing replay boundaries.
func openAIStreamOnlyWhitespaceText(data, eventType string) bool {
	if !gjson.Valid(data) {
		return false
	}
	emptyText := func(text gjson.Result) bool {
		return text.Type == gjson.String && strings.TrimSpace(text.String()) == ""
	}
	switch eventType {
	case "response.output_text.delta":
		return emptyText(gjson.Get(data, "delta"))
	case "response.output_text.done":
		return emptyText(gjson.Get(data, "text"))
	case "response.content_part.added", "response.content_part.done":
		part := gjson.Get(data, "part")
		return part.Get("type").String() == "output_text" && emptyText(part.Get("text"))
	case "response.output_item.added", "response.output_item.done":
		item := gjson.Get(data, "item")
		content := item.Get("content")
		if item.Get("type").String() != "message" || !content.IsArray() {
			return false
		}
		for _, part := range content.Array() {
			if part.Get("type").String() != "output_text" || !emptyText(part.Get("text")) {
				return false
			}
		}
		return true
	}
	return false
}
