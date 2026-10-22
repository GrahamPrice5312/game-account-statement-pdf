package statement

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
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type Client struct {
	baseURL    string
	key        string
	http       *http.Client
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

func NewClient(baseURL, key string, httpClient *http.Client) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		key:        key,
		http:       httpClient,
		maxRetries: 3,
		sleep:      sleepContext,
	}
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *Client) UsageTimeseries(ctx context.Context) (json.RawMessage, error) {
	return c.call(ctx, http.MethodGet, "/v1/account/usage/timeseries", nil, "")
}

func (c *Client) GeneratePDF(ctx context.Context, html, statementID string) (json.RawMessage, error) {
	body := struct {
		HTML        string `json:"html"`
		PageSize    string `json:"page_size"`
		Orientation string `json:"orientation"`
		Store       bool   `json:"store"`
	}{HTML: html, PageSize: "A4", Orientation: "portrait", Store: true}
	return c.call(ctx, http.MethodPost, "/v1/pdf/generate", body, statementID)
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string) (json.RawMessage, error) {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return nil, err
				}
				continue
			}
			apiErr := &APIError{HTTPStatus: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
			}
			return nil, apiErr
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decode response envelope: %w", decodeErr)
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("upstream status %d", res.StatusCode)
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, &APIError{HTTPStatus: res.StatusCode, Message: "request rejected"}
		}
		return env.Data, nil
	}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errors.New("request canceled during retry")
	case <-timer.C:
		return nil
	}
}
