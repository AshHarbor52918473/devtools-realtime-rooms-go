package infrai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestPublishReadsBusinessEnvelopeBeforeStatus(t *testing.T) {
	client := NewClient("test-key")
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s", req.Method)
		}
		if req.Header.Get("Idempotency-Key") != "evt-42" {
			t.Fatalf("missing idempotency key")
		}
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":false,"data":null,"error":{"code":"ROOM_INPUT_REJECTED","message":"check room input"},"metadata":{}}`)),
		}, nil
	})}

	_, err := client.Publish(context.Background(), PublishRequest{Channel: "build-main", Event: "build.completed", Data: map[string]string{"id": "evt-42"}}, "evt-42")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if apiErr.Code != "ROOM_INPUT_REJECTED" || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected API error: %#v", apiErr)
	}
}
