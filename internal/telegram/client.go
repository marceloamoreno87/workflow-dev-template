// internal/telegram/client.go
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

const responseCap = 1024 * 1024

func NewClient(baseURL, token string) (Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Client{}, fmt.Errorf("%w: base url needs http(s) with host", ErrTelegram)
	}
	if u.User != nil {
		return Client{}, fmt.Errorf("%w: credentials do not belong in urls", ErrTelegram)
	}
	if token == "" {
		return Client{}, fmt.Errorf("%w: token required", ErrTelegram)
	}
	return Client{
		baseURL: strings.TrimSuffix(u.Scheme+"://"+u.Host+u.Path, "/"),
		token:   token,
		client:  &http.Client{},
	}, nil
}

type RawUpdate struct {
	UpdateID int64           `json:"update_id"`
	Message  json.RawMessage `json:"message"`
}

func (c Client) call(ctx context.Context, method, action string, query url.Values, payload any) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("%w: encode", ErrTelegram)
		}
		bodyReader = bytes.NewReader(raw)
	}
	target := c.baseURL + "/bot" + c.token + "/" + action
	if query != nil {
		target += "?" + query.Encode()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("%w: build request", ErrTelegram)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: call failed", ErrTelegram)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("%w: status %d", ErrTelegram, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, responseCap+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read body", ErrTelegram)
	}
	if len(body) > responseCap {
		return nil, fmt.Errorf("%w: response too large", ErrTelegram)
	}
	return body, nil
}

func (c Client) GetUpdates(ctx context.Context, offset int64, timeoutSecs int) ([]RawUpdate, error) {
	query := url.Values{}
	if offset > 0 {
		query.Set("offset", strconv.FormatInt(offset, 10))
	}
	query.Set("timeout", strconv.Itoa(timeoutSecs))
	query.Set("limit", "100")
	body, err := c.call(ctx, "GET", "getUpdates", query, nil)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		OK     bool        `json:"ok"`
		Result []RawUpdate `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%w: decode updates", ErrTelegram)
	}
	if !envelope.OK {
		return nil, fmt.Errorf("%w: telegram refused", ErrTelegram)
	}
	return envelope.Result, nil
}

func (c Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	if n := len([]rune(strings.TrimSpace(text))); n == 0 || n > 4096 {
		return fmt.Errorf("%w: text length %d", ErrTelegram, n)
	}
	body, err := c.call(ctx, "POST", "sendMessage", nil, map[string]any{
		"chat_id": chatID, "text": text,
	})
	if err != nil {
		return err
	}
	var envelope struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("%w: decode send", ErrTelegram)
	}
	if !envelope.OK {
		return fmt.Errorf("%w: telegram refused", ErrTelegram)
	}
	return nil
}
