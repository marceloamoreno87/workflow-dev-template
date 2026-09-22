# Coolify Delivery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy releases through a typed Capability Broker that only accepts feature-disabled deploys, reports structured health evidence, and rolls back on demand — with credentials confined to one header and every failure free of secrets.

**Architecture:** `coolify` owns broker construction, request validation, deployment/health/rollback calls over HTTP, and response parsing with strict caps; it never shells out and never contacts the network except through its configured base URL. It may import nothing outside the standard library. All other modules stay untouched with no new imports. Registry authentication against the real upstream Coolify API, production TLS policy, and daemon-side deploy choreography belong to later increments and are explicitly out of scope: the paths below are the harness-broker v1 contract, not the upstream Coolify API.

**Tech Stack:** Go 1.27.1 standard library only (`net/http`, `net/http/httptest`, `encoding/json`, `context`, `io`, table-driven tests)

**Spec:** `docs/workflow.md` (delivery sequence: deploy Feature disabled, health and smoke evidence, rollback with possible data loss as a Human Gate), `docs/security.md` (Coolify overreach controlled by separate least-privilege tokens behind a typed Capability Broker; production logs begin as confidential), `docs/architecture.md` (Module Delivery; repository shape `internal/coolify`; no utils/common/manager), `docs/adr/0004-broker-coolify-capabilities.md`

## Global Constraints

- Use Go 1.27.1.
- Standard library only; do not add dependencies.
- `internal/coolify` imports nothing outside the standard library.
- Use the canonical terms from `CONTEXT.md` (Capability Broker, Feature, Project, Execution, Evidence, Human Gate); never write task or ticket for a Work Item.
- Deploys must carry the Feature disabled: `FeatureEnabled=true` is rejected before any HTTP call because exposure is a separate fact owned by the flags increment.
- Rollback requires a non-empty reason (auditable, Human-Gated upstream); the broker records the reason but never authorizes it.
- The token travels in exactly one `Authorization: Bearer` header; no credential ever appears in URLs (userinfo rejected), error strings, or request logs.
- HTTP responses are capped at 1 MiB; rejects anything larger before parsing. Context deadlines bound every call.
- Health evidence with zero checks is rejected as meaningless; health verdicts report, they do not decide (deploy/rollback decisions stay upstream).
- Unit tests run against `httptest` stub servers only; any live-broker test runs solely under `HARNESS_COOLIFY_URL` plus `HARNESS_COOLIFY_TOKEN` and skips otherwise.
- Do not add generic repository, provider, manager, service, utils, or common packages; new code lives in `internal/coolify` only.

---

### Task 1: Validate broker requests

**Files:**
- Create: `internal/coolify/broker.go`
- Test: `internal/coolify/broker_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `Broker struct`, `NewBroker(baseURL, token string) (Broker, error)`, `DeployRequest struct`, `RollbackRequest struct`, `(DeployRequest).Validate() error`, `(RollbackRequest).Validate() error`, sentinel `ErrCoolify`.

- [ ] **Step 1: Write the failing broker test**

```go
package coolify

import (
	"strings"
	"testing"
)

func validDeploy() DeployRequest {
	return DeployRequest{
		ResourceID:     "resource-1",
		ReleaseTag:     "v1.2.3",
		ReleaseCommit:  strings.Repeat("f", 40),
		FeatureEnabled: false,
	}
}

func TestValidateDeploy(t *testing.T) {
	t.Parallel()

	if err := validDeploy().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadDeploys(t *testing.T) {
	t.Parallel()

	mk := func(mut func(*DeployRequest)) DeployRequest {
		req := validDeploy()
		mut(&req)
		return req
	}
	for _, tc := range []struct {
		name string
		req  DeployRequest
	}{
		{name: "empty resource", req: mk(func(r *DeployRequest) { r.ResourceID = "" })},
		{name: "bad resource chars", req: mk(func(r *DeployRequest) { r.ResourceID = "res ource!" })},
		{name: "bad tag", req: mk(func(r *DeployRequest) { r.ReleaseTag = "1.2.3" })},
		{name: "bad commit", req: mk(func(r *DeployRequest) { r.ReleaseCommit = "abc" })},
		{name: "feature enabled", req: mk(func(r *DeployRequest) { r.FeatureEnabled = true })},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.req.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestValidateRollback(t *testing.T) {
	t.Parallel()

	req := RollbackRequest{ResourceID: "resource-1", ToDeploymentID: "dep-9", Reason: "health checks red"}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	empty := req
	empty.Reason = ""
	if err := empty.Validate(); err == nil {
		t.Fatal("expected reason requirement, got none")
	}
}

func TestRejectBadBrokers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		baseURL string
		token   string
	}{
		{name: "empty url", baseURL: "", token: "tok"},
		{name: "no scheme", baseURL: "broker.local", token: "tok"},
		{name: "userinfo", baseURL: "https://user:pass@broker.local", token: "tok"},
		{name: "empty token", baseURL: "https://broker.local", token: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewBroker(tc.baseURL, tc.token); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/coolify -run 'TestValidateDeploy|TestRejectBadDeploys|TestValidateRollback|TestRejectBadBrokers' -count=1`

Expected: FAIL because `Broker`, `NewBroker`, the request structs, and `ErrCoolify` are undefined.

- [ ] **Step 3: Implement broker construction and request validation**

```go
// internal/coolify/broker.go
package coolify

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

var ErrCoolify = errors.New("invalid coolify request")

var resourcePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

var tagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type Broker struct {
	baseURL string
	token   string
}

func NewBroker(baseURL, token string) (Broker, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Broker{}, fmt.Errorf("%w: base url needs http(s) with host", ErrCoolify)
	}
	if u.User != nil {
		return Broker{}, fmt.Errorf("%w: credentials do not belong in urls", ErrCoolify)
	}
	if token == "" {
		return Broker{}, fmt.Errorf("%w: token required", ErrCoolify)
	}
	return Broker{baseURL: u.Scheme + "://" + u.Host + u.Path, token: token}, nil
}

type DeployRequest struct {
	ResourceID     string
	ReleaseTag     string
	ReleaseCommit  string
	FeatureEnabled bool
}

func (r DeployRequest) Validate() error {
	if !resourcePattern.MatchString(r.ResourceID) {
		return fmt.Errorf("%w: resource id", ErrCoolify)
	}
	if !tagPattern.MatchString(r.ReleaseTag) {
		return fmt.Errorf("%w: release tag %q", ErrCoolify, r.ReleaseTag)
	}
	if !shaPattern.MatchString(r.ReleaseCommit) {
		return fmt.Errorf("%w: release commit", ErrCoolify)
	}
	if r.FeatureEnabled {
		return fmt.Errorf("%w: deploy must land with the feature disabled", ErrCoolify)
	}
	return nil
}

type RollbackRequest struct {
	ResourceID     string
	ToDeploymentID string
	Reason         string
}

func (r RollbackRequest) Validate() error {
	if !resourcePattern.MatchString(r.ResourceID) {
		return fmt.Errorf("%w: resource id", ErrCoolify)
	}
	if !resourcePattern.MatchString(r.ToDeploymentID) {
		return fmt.Errorf("%w: deployment id", ErrCoolify)
	}
	if r.Reason == "" {
		return fmt.Errorf("%w: rollback reason required", ErrCoolify)
	}
	if len([]rune(r.Reason)) > 2000 {
		return fmt.Errorf("%w: rollback reason too long", ErrCoolify)
	}
	return nil
}
```

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/coolify -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/coolify/broker.go internal/coolify/broker_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/coolify/broker.go internal/coolify/broker_test.go
git commit -m "feat(coolify): validate brokered requests"
```

### Task 2: Deploy through the broker against a stub server

**Files:**
- Create: `internal/coolify/deploy.go`
- Test: `internal/coolify/deploy_test.go`

**Interfaces:**
- Consumes: `Broker`, `DeployRequest` from Task 1.
- Produces: `Deployment struct`, `(Broker).Deploy(ctx context.Context, req DeployRequest) (Deployment, error)`, `POST /api/v1/deployments`, 1 MiB response cap, secret-free errors.

- [ ] **Step 1: Write the failing deploy test**

```go
package coolify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeployRoundTrip(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deploymentId":"dep-1","status":"deploying"}`))
	}))
	defer server.Close()

	broker, err := NewBroker(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	dep, err := broker.Deploy(context.Background(), validDeploy())
	if err != nil {
		t.Fatal(err)
	}
	if dep.ID != "dep-1" || dep.Status != "deploying" {
		t.Fatalf("unexpected deployment: %#v", dep)
	}
	if gotMethod != "POST" || gotPath != "/api/v1/deployments" {
		t.Fatalf("unexpected call: %s %s", gotMethod, gotPath)
	}
	if gotAuth != "Bearer stub-token" {
		t.Fatalf("auth header wrong: %q", gotAuth)
	}
	for _, key := range []string{"resourceId", "releaseTag", "releaseCommit"} {
		if _, ok := gotBody[key]; !ok {
			t.Fatalf("body missing %q: %v", key, gotBody)
		}
	}
	if _, ok := gotBody["featureEnabled"]; ok {
		t.Fatalf("feature flag state is not broker business: %v", gotBody)
	}
}

func TestDeployRejectsBeforeCalling(t *testing.T) {
	t.Parallel()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	defer server.Close()

	broker, err := NewBroker(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	bad := validDeploy()
	bad.FeatureEnabled = true
	if _, err := broker.Deploy(context.Background(), bad); err == nil {
		t.Fatal("expected rejection, got none")
	}
	if calls != 0 {
		t.Fatal("invalid request reached the server")
	}
}

func TestDeployErrorsHideSecrets(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	broker, err := NewBroker(server.URL, "s3cr3t-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Deploy(context.Background(), validDeploy()); err == nil {
		t.Fatal("expected failure, got none")
	} else if strings.Contains(err.Error(), "s3cr3t-token") {
		t.Fatalf("secret leaked into error: %v", err)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/coolify -run 'TestDeploy' -count=1`

Expected: FAIL because `Deploy` and `Deployment` are undefined.

- [ ] **Step 3: Implement Deploy with caps and secret-free errors**

```go
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
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/coolify -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/coolify/deploy.go internal/coolify/deploy_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/coolify/deploy.go internal/coolify/deploy_test.go
git commit -m "feat(coolify): deploy through the broker"
```

### Task 3: Report health and roll back through the broker

**Files:**
- Create: `internal/coolify/health.go`
- Test: `internal/coolify/health_test.go`

**Interfaces:**
- Consumes: `Broker.post` from Task 2.
- Produces: `HealthCheck struct`, `HealthEvidence struct`, `(Broker).Health(ctx, deploymentID) (HealthEvidence, error)` via `GET /api/v1/deployments/{id}/health`, `(Broker).Rollback(ctx, req) (Deployment, error)` via `POST /api/v1/rollbacks`.

- [ ] **Step 1: Write the failing health test**

```go
package coolify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthRoundTrip(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deploymentId":"dep-1","healthy":true,"checks":[{"name":"smoke","passed":true},{"name":"http","passed":true}]}`))
	}))
	defer server.Close()

	broker, err := NewBroker(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := broker.Health(context.Background(), "dep-1")
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Healthy || len(evidence.Checks) != 2 || evidence.DeploymentID != "dep-1" {
		t.Fatalf("unexpected evidence: %#v", evidence)
	}
	if gotPath != "/api/v1/deployments/dep-1/health" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
}

func TestHealthRejectsEmptyChecks(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"deploymentId":"dep-1","healthy":true,"checks":[]}`))
	}))
	defer server.Close()

	broker, err := NewBroker(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Health(context.Background(), "dep-1"); err == nil {
		t.Fatal("expected empty-checks rejection, got none")
	}
	if _, err := broker.Health(context.Background(), ""); err == nil {
		t.Fatal("expected id rejection, got none")
	}
}

func TestRollbackRoundTrip(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rollbacks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deploymentId":"dep-8","status":"rolling-back"}`))
	}))
	defer server.Close()

	broker, err := NewBroker(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	dep, err := broker.Rollback(context.Background(), RollbackRequest{ResourceID: "resource-1", ToDeploymentID: "dep-9", Reason: "health checks red"})
	if err != nil {
		t.Fatal(err)
	}
	if dep.ID != "dep-8" {
		t.Fatalf("unexpected rollback: %#v", dep)
	}
	if _, err := broker.Rollback(context.Background(), RollbackRequest{ResourceID: "resource-1", ToDeploymentID: "dep-9"}); err == nil {
		t.Fatal("expected reason requirement, got none")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/coolify -run 'TestHealth|TestRollback' -count=1`

Expected: FAIL because `Health`, `HealthEvidence`, `HealthCheck`, and `Rollback` are undefined.

- [ ] **Step 3: Implement Health and Rollback**

```go
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
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/coolify -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/coolify/health.go internal/coolify/health_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/coolify/health.go internal/coolify/health_test.go
git commit -m "feat(coolify): report health and roll back"
```

### Task 4: Prove the deploy-health-rollback flow with caps and timeouts

**Files:**
- Create: `internal/coolify/flow_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete broker from Tasks 1-3.
- Produces: executable evidence of the full flow against one stub server, oversized-response rejection, deadline enforcement, and the env-gated live path that skips by default.

- [ ] **Step 1: Add the flow test**

```go
package coolify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDeliverFlow(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/deployments":
			_, _ = w.Write([]byte(`{"deploymentId":"dep-1","status":"deploying"}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/deployments/dep-1/health":
			_, _ = w.Write([]byte(`{"deploymentId":"dep-1","healthy":false,"checks":[{"name":"smoke","passed":false}]}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/rollbacks":
			_, _ = w.Write([]byte(`{"deploymentId":"dep-0","status":"rolling-back"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	broker, err := NewBroker(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	dep, err := broker.Deploy(ctx, validDeploy())
	if err != nil || dep.ID != "dep-1" {
		t.Fatalf("deploy: %#v %v", dep, err)
	}
	evidence, err := broker.Health(ctx, dep.ID)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Healthy {
		t.Fatal("red health reported healthy")
	}
	rolled, err := broker.Rollback(ctx, RollbackRequest{ResourceID: "resource-1", ToDeploymentID: "dep-0", Reason: "smoke red"})
	if err != nil || rolled.ID != "dep-0" {
		t.Fatalf("rollback: %#v %v", rolled, err)
	}
}

func TestOversizedResponseRejected(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deploymentId":"` + strings.Repeat("d", responseCap) + `"}`))
	}))
	defer server.Close()

	broker, err := NewBroker(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Deploy(context.Background(), validDeploy()); err == nil {
		t.Fatal("expected oversized rejection, got none")
	}
}

func TestDeadlineEnforced(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	broker, err := NewBroker(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := broker.Deploy(ctx, validDeploy()); err == nil {
		t.Fatal("expected deadline failure, got none")
	}
}

func TestLiveBrokerIsGated(t *testing.T) {
	if os.Getenv("HARNESS_COOLIFY_URL") == "" || os.Getenv("HARNESS_COOLIFY_TOKEN") == "" {
		t.Skip("set HARNESS_COOLIFY_URL and HARNESS_COOLIFY_TOKEN to run against a live broker")
	}
}
```

NOTE: `TestLiveBrokerIsGated` needs the `os` import. It intentionally only asserts the gate: pointing the broker at a real server is an operator action with a real token and is never attempted by default.

- [ ] **Step 2: Run the flow test**

Run: `go test ./internal/coolify -run 'TestDeliverFlow|TestOversized|TestDeadline|TestLiveBroker' -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean build of `cmd/harness`.

- [ ] **Step 4: Mark Increment 10 verified and commit**

In `docs/implementation-plan.md`, add Increment 10 to the plan index and a verification checklist below Increment 9. Do not mark it complete until the commands above pass on master.

```bash
git add internal/coolify/flow_test.go docs/implementation-plan.md
git commit -m "test(coolify): verify deliver flow with caps"
```
