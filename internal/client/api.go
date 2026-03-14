package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	maxRetries     = 3
	baseBackoff    = 1 * time.Second
	maxBackoff     = 30 * time.Second
	backoffFactor  = 2.0
)

// Client communicates with the Upwatchly backend.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// SendMetrics posts a metrics report with retries and returns the server config.
func (c *Client) SendMetrics(ctx context.Context, report *MetricsReport) (*ServerConfig, error) {
	body, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(float64(baseBackoff) * math.Pow(backoffFactor, float64(attempt-1)))
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			log.WithFields(log.Fields{
				"attempt": attempt + 1,
				"backoff": backoff,
			}).Warn("retrying metrics send")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		config, err := c.doSend(ctx, body)
		if err == nil {
			return config, nil
		}
		lastErr = err

		// Don't retry on client errors (4xx) except 429
		if isClientError(lastErr) {
			return nil, lastErr
		}
	}

	return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) doSend(ctx context.Context, body []byte) (*ServerConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/metrics", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-agent-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.WithFields(log.Fields{
			"status": resp.StatusCode,
			"body":   string(respBody),
		}).Warn("backend returned non-2xx")
		return nil, &apiError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var config ServerConfig
	if err := json.Unmarshal(respBody, &config); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	return &config, nil
}

type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("backend error: status %d", e.StatusCode)
}

func isClientError(err error) bool {
	if ae, ok := err.(*apiError); ok {
		return ae.StatusCode >= 400 && ae.StatusCode < 500 && ae.StatusCode != 429
	}
	return false
}
