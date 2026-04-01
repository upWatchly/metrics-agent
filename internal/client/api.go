package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/http/httptrace"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	maxRetries    = 3
	baseBackoff   = 1 * time.Second
	maxBackoff    = 30 * time.Second
	backoffFactor = 2.0
)

// Client communicates with the Upwatchly backend.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// New creates a new API client.
func New(baseURL, token string, disableKeepAlive bool) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives:   disableKeepAlive,
				TLSHandshakeTimeout: 10 * time.Second,
			},
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
			// Add ±25% jitter to avoid thundering herd
			jitter := 0.75 + rand.Float64()*0.5 // [0.75, 1.25)
			backoff = time.Duration(float64(backoff) * jitter)
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
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+"/v1/servers/report", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Server-Token", c.token)

	var dnsStart, connStart, tlsStart time.Time
	trace := &httptrace.ClientTrace{
		DNSStart:     func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:      func(_ httptrace.DNSDoneInfo) { log.WithField("dur", time.Since(dnsStart)).Debug("http: dns") },
		ConnectStart: func(_, _ string) { connStart = time.Now() },
		ConnectDone: func(_, _ string, err error) {
			f := log.Fields{"dur": time.Since(connStart)}
			if err != nil {
				f["error"] = err.Error()
			}
			log.WithFields(f).Debug("http: connect")
		},
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone:  func(_ tls.ConnectionState, _ error) { log.WithField("dur", time.Since(tlsStart)).Debug("http: tls") },
		GotConn: func(info httptrace.GotConnInfo) {
			log.WithFields(log.Fields{"reused": info.Reused, "idle": info.IdleTime}).Debug("http: got conn")
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(reqCtx, trace))

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "dur": time.Since(start)}).Debug("http: request failed")
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
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
