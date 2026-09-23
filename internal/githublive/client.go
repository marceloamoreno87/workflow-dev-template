// internal/githublive/client.go
package githublive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrGitHub = errors.New("github request failed")

const responseCap = 1024 * 1024

const maxAttempts = 3

type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

type staticToken string

func StaticToken(token string) TokenProvider {
	return staticToken(token)
}

func (s staticToken) Token(context.Context) (string, error) {
	if s == "" {
		return "", fmt.Errorf("%w: empty token", ErrGitHub)
	}
	return string(s), nil
}

type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewClient(baseURL, token string) (Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Client{}, fmt.Errorf("%w: base url needs http(s) with host", ErrGitHub)
	}
	if u.User != nil {
		return Client{}, fmt.Errorf("%w: credentials do not belong in urls", ErrGitHub)
	}
	if token == "" {
		return Client{}, fmt.Errorf("%w: token required", ErrGitHub)
	}
	return Client{
		baseURL: strings.TrimSuffix(u.Scheme+"://"+u.Host+u.Path, "/"),
		token:   token,
		client:  &http.Client{},
	}, nil
}

// Call records one request/response line for evidence: method, path, status.
type Call struct {
	Method     string
	Path       string
	StatusCode int
}

func retryable(status int) bool {
	if status == http.StatusTooManyRequests {
		return true
	}
	switch status {
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func backoff(attempt int, res *http.Response) time.Duration {
	if res != nil {
		if after := res.Header.Get("Retry-After"); after != "" {
			var seconds int
			if _, err := fmt.Sscanf(after, "%d", &seconds); err == nil && seconds >= 0 {
				if seconds > 1 {
					seconds = 1
				}
				return time.Duration(seconds) * time.Second
			}
		}
	}
	return time.Duration(50*attempt) * time.Millisecond
}

func (c Client) roundTrip(ctx context.Context, method, path, rawQuery string, payload any) ([]byte, Call, error) {
	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, Call{}, fmt.Errorf("%w: encode", ErrGitHub)
		}
		bodyReader = bytes.NewReader(raw)
	}
	target := c.baseURL + path
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	call := Call{Method: method, Path: path}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
		if err != nil {
			return nil, call, fmt.Errorf("%w: build request", ErrGitHub)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+c.token)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.client.Do(req)
		if err != nil {
			return nil, call, fmt.Errorf("%w: call failed", ErrGitHub)
		}
		call.StatusCode = res.StatusCode
		if res.StatusCode >= 200 && res.StatusCode <= 299 {
			body, err := io.ReadAll(io.LimitReader(res.Body, responseCap+1))
			res.Body.Close()
			if err != nil {
				return nil, call, fmt.Errorf("%w: read body", ErrGitHub)
			}
			if len(body) > responseCap {
				return nil, call, fmt.Errorf("%w: response too large", ErrGitHub)
			}
			return body, call, nil
		}
		res.Body.Close()
		if !retryable(res.StatusCode) || attempt == maxAttempts {
			return nil, call, fmt.Errorf("%w: status %d", ErrGitHub, res.StatusCode)
		}
		lastErr = fmt.Errorf("%w: status %d", ErrGitHub, res.StatusCode)
		timer := time.NewTimer(backoff(attempt, res))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, call, fmt.Errorf("%w: call failed", ErrGitHub)
		case <-timer.C:
		}
	}
	return nil, call, lastErr
}

func (c Client) get(ctx context.Context, path, rawQuery string) ([]byte, Call, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.roundTrip(ctx, "GET", path, rawQuery, nil)
}

func (c Client) post(ctx context.Context, path string, payload any) ([]byte, Call, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.roundTrip(ctx, "POST", path, "", payload)
}

func (c Client) put(ctx context.Context, path string, payload any) ([]byte, Call, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.roundTrip(ctx, "PUT", path, "", payload)
}
