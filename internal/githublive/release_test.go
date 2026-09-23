package githublive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCreateRelease(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases" {
			t.Errorf("path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":7,"tag_name":"v1.2.3"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	rel, call, err := client.CreateRelease(context.Background(), "owner", "repo", "v1.2.3", strings.Repeat("f", 40), "v1.2.3", "Immutable release.")
	if err != nil {
		t.Fatal(err)
	}
	if rel.ID != 7 || rel.Tag != "v1.2.3" || call.StatusCode != 201 {
		t.Fatalf("unexpected release: %#v %#v", rel, call)
	}
	if _, _, err := client.CreateRelease(context.Background(), "owner", "repo", "1.2.3", strings.Repeat("f", 40), "x", "y"); err == nil {
		t.Fatal("expected tag rejection, got none")
	}
}

func TestListCommitsDetectsForeign(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/pulls/12/commits" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"sha":"` + strings.Repeat("a", 40) + `","commit":{"author":{"name":"harness"}},"author":{"login":"harness-bot"}},{"sha":"` + strings.Repeat("b", 40) + `","commit":{"author":{"name":"contributor"}},"author":{"login":"octocat"}}]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	commits, _, err := client.ListCommits(context.Background(), "owner", "repo", 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("unexpected commits: %#v", commits)
	}
	if !HasForeignCommits(commits, "harness-bot") {
		t.Fatal("foreign commit missed")
	}
	if HasForeignCommits(commits[:1], "harness-bot") {
		t.Fatal("own commit flagged foreign")
	}
}

func TestRetryThenSucceeds(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"id":7,"tag_name":"v1.2.3"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.CreateRelease(context.Background(), "owner", "repo", "v1.2.3", strings.Repeat("f", 40), "v1.2.3", "x"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls.Load())
	}
}

func TestNoRetryOnClientErrors(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.CreateRelease(context.Background(), "owner", "repo", "v1.2.3", strings.Repeat("f", 40), "v1.2.3", "x"); err == nil {
		t.Fatal("expected failure, got none")
	}
	if calls.Load() != 1 {
		t.Fatalf("client errors must not retry: %d", calls.Load())
	}
}
