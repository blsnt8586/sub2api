package service

import (
	"errors"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

// normalizeSub2APIProviderBaseURL converts the value entered in the admin
// form into the site root used by the Sub2API client. Operators commonly copy
// the Keys page or the Keys API endpoint from the upstream browser; those
// known suffixes are safe to remove while preserving a real deployment
// prefix, such as https://example.com/sub2api/keys.
func normalizeSub2APIProviderBaseURL(raw string) (string, error) {
	normalized, err := urlvalidator.ValidateURLFormat(strings.TrimSpace(raw), true)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("base URL must not contain credentials, query parameters, or a fragment")
	}

	path := strings.TrimRight(parsed.Path, "/")
	for _, suffix := range []string{"/api/v1/keys", "/api/v1", "/keys"} {
		if path == suffix {
			path = ""
			break
		}
		if strings.HasSuffix(path, suffix) {
			path = strings.TrimSuffix(path, suffix)
			break
		}
	}
	parsed.Path = strings.TrimRight(path, "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}
