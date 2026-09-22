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
