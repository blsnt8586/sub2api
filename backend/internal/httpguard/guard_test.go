package httpguard_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/httpguard"
)

func TestValidateURL_RejectsPrivateRanges(t *testing.T) {
	g := httpguard.New(httpguard.Config{AllowedHosts: nil})
	cases := []struct {
		url     string
		wantErr bool
		label   string
	}{
		{"http://169.254.169.254/latest/meta-data", true, "cloud metadata"},
		{"http://10.0.0.1/internal", true, "private 10.x"},
		{"http://192.168.1.1/", true, "private 192.168.x"},
		{"http://172.16.0.5/", true, "private 172.16.x"},
		{"http://127.0.0.1/", true, "loopback"},
		{"http://[::1]/", true, "loopback IPv6"},
		{"ftp://example.com/", true, "non-http scheme"},
		{"http://example.com/v1", false, "public host"},
		{"https://example.com/v1", false, "public HTTPS"},
	}
	for _, tc := range cases {
		err := g.ValidateURL(tc.url)
		if tc.wantErr && err == nil {
			t.Errorf("[%s] expected error for %q, got nil", tc.label, tc.url)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("[%s] unexpected error for %q: %v", tc.label, tc.url, err)
		}
	}
}

func TestValidateURL_AllowsWhitelisted(t *testing.T) {
	g := httpguard.New(httpguard.Config{AllowedHosts: []string{"localhost", "127.0.0.1"}})
	if err := g.ValidateURL("http://localhost:8080/v1/tasks/images"); err != nil {
		t.Errorf("whitelisted localhost should be allowed, got: %v", err)
	}
	if err := g.ValidateURL("http://127.0.0.1:8080/api"); err != nil {
		t.Errorf("whitelisted 127.0.0.1 should be allowed, got: %v", err)
	}
	// Other private addresses still blocked even when allowlist set
	if err := g.ValidateURL("http://10.0.0.1/"); err == nil {
		t.Error("non-whitelisted 10.0.0.1 should be blocked")
	}
}

func TestHTTPClientBlocksPrivate(t *testing.T) {
	// httptest.NewServer binds to 127.0.0.1 - guard should block it by default
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "secret")
	}))
	defer srv.Close()

	g := httpguard.New(httpguard.Config{AllowedHosts: nil})
	client := g.HTTPClient()
	_, err := client.Get(srv.URL)
	if err == nil {
		t.Error("HTTP client should block loopback address")
	}
}

func TestHTTPClientAllowsWhitelistedHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Extract the host:port from the test server URL
	host, _, _ := net.SplitHostPort(srv.Listener.Addr().String())

	g := httpguard.New(httpguard.Config{AllowedHosts: []string{host}})
	client := g.HTTPClient()
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Errorf("whitelisted host should be allowed: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
}
