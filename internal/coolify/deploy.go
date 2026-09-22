// internal/coolify/deploy.go
package coolify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const responseCap = 1024 * 1024

type Deployment struct {
	ID     string
	Status string
}

func (b Broker) post(ctx context.Context, path string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: encode", ErrCoolify)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: build request", ErrCoolify)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: call failed", ErrCoolify)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("%w: status %d", ErrCoolify, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, responseCap+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read body", ErrCoolify)
	}
	if len(body) > responseCap {
		return nil, fmt.Errorf("%w: response too large", ErrCoolify)
	}
	return body, nil
}

func (b Broker) Deploy(ctx context.Context, req DeployRequest) (Deployment, error) {
	if err := req.Validate(); err != nil {
		return Deployment{}, err
	}
	body, err := b.post(ctx, "/api/v1/deployments", map[string]string{
		"resourceId":    req.ResourceID,
		"releaseTag":    req.ReleaseTag,
		"releaseCommit": req.ReleaseCommit,
	})
	if err != nil {
		return Deployment{}, err
	}
	var doc struct {
		DeploymentID string `json:"deploymentId"`
		Status       string `json:"status"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return Deployment{}, fmt.Errorf("%w: decode deployment", ErrCoolify)
	}
	if !resourcePattern.MatchString(doc.DeploymentID) || doc.Status == "" {
		return Deployment{}, fmt.Errorf("%w: malformed deployment", ErrCoolify)
	}
	return Deployment{ID: doc.DeploymentID, Status: doc.Status}, nil
}
