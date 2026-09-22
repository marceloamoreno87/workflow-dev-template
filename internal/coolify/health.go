// internal/coolify/health.go
package coolify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HealthCheck struct {
	Name   string
	Passed bool
}

type HealthEvidence struct {
	DeploymentID string
	Healthy      bool
	Checks       []HealthCheck
}

func (b Broker) get(ctx context.Context, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request", ErrCoolify)
	}
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

func (b Broker) Health(ctx context.Context, deploymentID string) (HealthEvidence, error) {
	if !resourcePattern.MatchString(deploymentID) {
		return HealthEvidence{}, fmt.Errorf("%w: deployment id", ErrCoolify)
	}
	body, err := b.get(ctx, "/api/v1/deployments/"+deploymentID+"/health")
	if err != nil {
		return HealthEvidence{}, err
	}
	var doc struct {
		DeploymentID string `json:"deploymentId"`
		Healthy      bool   `json:"healthy"`
		Checks       []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return HealthEvidence{}, fmt.Errorf("%w: decode health", ErrCoolify)
	}
	if doc.DeploymentID == "" || len(doc.Checks) == 0 {
		return HealthEvidence{}, fmt.Errorf("%w: health without checks", ErrCoolify)
	}
	evidence := HealthEvidence{DeploymentID: doc.DeploymentID, Healthy: doc.Healthy}
	for _, c := range doc.Checks {
		if c.Name == "" {
			return HealthEvidence{}, fmt.Errorf("%w: unnamed check", ErrCoolify)
		}
		evidence.Checks = append(evidence.Checks, HealthCheck{Name: c.Name, Passed: c.Passed})
	}
	return evidence, nil
}

func (b Broker) Rollback(ctx context.Context, req RollbackRequest) (Deployment, error) {
	if err := req.Validate(); err != nil {
		return Deployment{}, err
	}
	body, err := b.post(ctx, "/api/v1/rollbacks", map[string]string{
		"resourceId":     req.ResourceID,
		"toDeploymentId": req.ToDeploymentID,
		"reason":         req.Reason,
	})
	if err != nil {
		return Deployment{}, err
	}
	var doc struct {
		DeploymentID string `json:"deploymentId"`
		Status       string `json:"status"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return Deployment{}, fmt.Errorf("%w: decode rollback", ErrCoolify)
	}
	if !resourcePattern.MatchString(doc.DeploymentID) || doc.Status == "" {
		return Deployment{}, fmt.Errorf("%w: malformed rollback", ErrCoolify)
	}
	return Deployment{ID: doc.DeploymentID, Status: doc.Status}, nil
}
