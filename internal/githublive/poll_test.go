package githublive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPollMapsIssues(t *testing.T) {
	t.Parallel()

	var gotSince, gotState, gotPerPage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/issues" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer stub-token" {
			t.Errorf("auth wrong: %q", r.Header.Get("Authorization"))
		}
		query := r.URL.Query()
		gotSince, gotState, gotPerPage = query.Get("since"), query.Get("state"), query.Get("per_page")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
{"number":78,"title":"Add health endpoint","body":"Please.","state":"open","user":{"login":"octocat"},"updated_at":"2026-09-23T00:00:00Z"},
{"number":79,"title":"Closed item","body":null,"state":"closed","user":{"login":"octocat"},"updated_at":"2026-09-23T01:00:00Z"}
]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	since := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	issues, err := poller.Poll(context.Background(), since, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 || issues[0].Number != 78 || issues[0].Title != "Add health endpoint" {
		t.Fatalf("unexpected issues: %#v", issues)
	}
	perPage, _ := strconv.Atoi(gotPerPage)
	if gotState != "all" || perPage != 50 || !strings.Contains(gotSince, "2026-09-22") {
		t.Fatalf("query wrong: since=%q state=%q per_page=%q", gotSince, gotState, gotPerPage)
	}
}

func TestPollRejectsMalformed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":0,"title":"x","state":"open"}]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := poller.Poll(context.Background(), time.Now(), 10); err == nil {
		t.Fatal("expected malformed rejection, got none")
	}
}

func TestPollHidesSecrets(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "s3cr3t-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := poller.Poll(context.Background(), time.Now(), 10); err == nil {
		t.Fatal("expected failure, got none")
	} else if strings.Contains(err.Error(), "s3cr3t-token") {
		t.Fatalf("secret leaked: %v", err)
	}
}

func TestRejectBadPollers(t *testing.T) {
	t.Parallel()

	client, err := NewClient("https://api.example", "tok")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range [][2]string{{"", "repo"}, {"owner", ""}, {"bad owner!", "repo"}} {
		if _, err := NewPoller(client, tc[0], tc[1]); err == nil {
			t.Fatalf("expected rejection for %q, got none", tc)
		}
	}
}
