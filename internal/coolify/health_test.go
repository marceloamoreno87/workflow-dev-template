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
