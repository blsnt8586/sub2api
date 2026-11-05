package service

import (
	"fmt"
	"net/http"
)

// openAICommittedStreamError retains account-health attribution after output
// has been sent. It deliberately does not unwrap to UpstreamFailoverError:
// cooling an unhealthy account must never replay already committed output.
type openAICommittedStreamError struct {
	cause    error
	status   int
	body     []byte
	eligible bool
}

func (e *openAICommittedStreamError) Error() string { return e.cause.Error() }
func (e *openAICommittedStreamError) Unwrap() error { return e.cause }

func newOpenAICommittedStreamFailure(payload []byte, message string) error {
	status := openAIStreamFailureStatus(payload, message)
	return &openAICommittedStreamError{
		cause:  fmt.Errorf("upstream response failed: %s", message),
		status: status,
		body:   append([]byte(nil), payload...),
		eligible: status >= http.StatusInternalServerError &&
			openAIStreamFailedEventShouldFailover(payload, message) &&
			!isOpenAIUpstreamCapacityShedEvent(payload),
	}
}

func newOpenAICommittedStreamReadError(cause error) error {
	return &openAICommittedStreamError{
		cause:    fmt.Errorf("stream read error: %w", cause),
		status:   http.StatusBadGateway,
		body:     []byte("Upstream stream read failed"),
		eligible: true,
	}
}
