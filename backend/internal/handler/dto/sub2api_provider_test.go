//go:build unit

package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProviderJSONIncludesClearedNotes(t *testing.T) {
	payload, err := json.Marshal(Provider{ID: 7, Notes: nil})
	if err != nil {
		t.Fatalf("marshal provider: %v", err)
	}
	if !strings.Contains(string(payload), `"notes":null`) {
		t.Fatalf("payload = %s, want explicit null notes", payload)
	}
}

func TestProviderJSONIncludesDirectProxyRoute(t *testing.T) {
	payload, err := json.Marshal(Provider{ID: 7, ProxyID: nil})
	if err != nil {
		t.Fatalf("marshal provider: %v", err)
	}
	if !strings.Contains(string(payload), `"proxy_id":null`) {
		t.Fatalf("payload = %s, want explicit null proxy_id", payload)
	}
}
