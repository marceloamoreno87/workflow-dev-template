package coolify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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
