package service

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// applyFreshAccountTestSessionHeaders isolates each synthetic OpenAI account
// test from earlier probes. A random window ID alone is not a sticky-routing
// signal: Sub2API falls back to the request content when session headers are
// absent, so repeated greetings or arithmetic questions can reuse a binding.
// Apply this after account header overrides, only on account-test requests.
func applyFreshAccountTestSessionHeaders(h http.Header) {
	if h == nil {
		return
	}
	sessionID := uuid.NewString()
	for _, name := range []string{"session-id", "session_id", "conversation_id", "x-codex-window-id"} {
		// Overrides can use wire casing rather than net/http canonical casing.
		for existing := range h {
			if strings.EqualFold(existing, name) {
				delete(h, existing)
			}
		}
		// The hyphenated alias survives proxies that drop underscore headers.
		h.Set(name, sessionID)
	}
}
