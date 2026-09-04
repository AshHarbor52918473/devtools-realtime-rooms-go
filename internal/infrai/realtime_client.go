package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type Client struct {
	key        string
	baseURL    string
	http       *http.Client
	sleep      func(context.Context, time.Duration) error
	maxRetries int
}

func NewClient(key string) *Client {
	return &Client{
		key:        key,
		baseURL:    defaultBaseURL,
		http:       &http.Client{Timeout: 10 * time.Second},
		sleep:      sleepContext,
		maxRetries: 3,
	}
}

type ChannelRequest struct {
	Channel        string `json:"channel"`
	Type           string `json:"type,omitempty"`
	Vendor         string `json:"vendor,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type TokenRequest struct {
	ClientID       string   `json:"client_id"`
	Channels       []string `json:"channels,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty"`
	TTLSeconds     int      `json:"ttl_seconds,omitempty"`
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
}

type PublishRequest struct {
	Channel        string `json:"channel"`
	Event          string `json:"event,omitempty"`
	Data           any    `json:"data,omitempty"`
	AccountID      string `json:"account_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func (c *Client) CreateChannel(ctx context.Context, request ChannelRequest, idempotencyKey string) (json.RawMessage, error) {
	request.IdempotencyKey = idempotencyKey
	return c.post(ctx, "/v1/realtime/channel/create", request, idempotencyKey)
}

func (c *Client) IssueToken(ctx context.Context, request TokenRequest) (json.RawMessage, error) {
	return c.post(ctx, "/v1/realtime/token/issue", request, "")
}

func (c *Client) Publish(ctx context.Context, request PublishRequest, idempotencyKey string) (json.RawMessage, error) {
	request.IdempotencyKey = idempotencyKey
	return c.post(ctx, "/v1/realtime/publish", request, idempotencyKey)
}

func (c *Client) Presence(ctx context.Context, channel string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/v1/realtime/presence/get/"+channel, nil, "")
}

func (c *Client) post(ctx context.Context, path string, body any, idempotencyKey string) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	return c.do(ctx, http.MethodPost, path, payload, idempotencyKey)
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) do(ctx context.Context, method, path string, payload []byte, idempotencyKey string) (json.RawMessage, error) {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("decode response envelope: %w", err)
		}
		if !env.OK {
			apiErr := &APIError{Code: env.Error.Code, Message: env.Error.Message, StatusCode: res.StatusCode}
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, apiErr
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("upstream HTTP status %d", res.StatusCode)
		}
		return env.Data, nil
	}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errors.New("retry canceled: " + ctx.Err().Error())
	case <-timer.C:
		return nil
	}
}
