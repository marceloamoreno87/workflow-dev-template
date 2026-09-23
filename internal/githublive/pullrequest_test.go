package githublive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenPR(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != "POST" {
			t.Errorf("method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":12,"head":{"sha":"` + strings.Repeat("d", 40) + `"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	pr, call, err := client.OpenPR(context.Background(), "owner", "repo", NewPR{Base: "main", Head: "feat-x", Title: "Add x", Body: "Evidence attached."})
	if err != nil {
		t.Fatal(err)
	}
	if pr.Number != 12 || call.Method != "POST" || gotPath != "/repos/owner/repo/pulls" {
		t.Fatalf("unexpected open: %#v %#v %s", pr, call, gotPath)
	}
	if pr.Head != strings.Repeat("d", 40) {
		t.Fatalf("head lost: %#v", pr)
	}
}

func TestMergeEnforcesMethod(t *testing.T) {
	t.Parallel()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/repos/owner/repo/pulls/12/merge" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"merged":true,"sha":"` + strings.Repeat("d", 40) + `"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"squash", "rebase", ""} {
		if _, _, err := client.MergePR(context.Background(), "owner", "repo", 12, method); err == nil {
			t.Fatalf("method %q reached the server", method)
		}
	}
	if calls != 0 {
		t.Fatal("non-merge method reached the server")
	}
	merge, _, err := client.MergePR(context.Background(), "owner", "repo", 12, "merge")
	if err != nil {
		t.Fatal(err)
	}
	if !merge.Merged {
		t.Fatal("merge not reported")
	}
}

func TestMergeRefusesUnmerged(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"merged":false,"message":"conflict"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.MergePR(context.Background(), "owner", "repo", 12, "merge"); err == nil {
		t.Fatal("expected unmerged failure, got none")
	}
}

func TestRejectBadPRs(t *testing.T) {
	t.Parallel()

	client, err := NewClient("https://api.example", "tok")
	if err != nil {
		t.Fatal(err)
	}
	_ = client
	for _, pr := range []NewPR{
		{Base: "", Head: "feat-x", Title: "t"},
		{Base: "main", Head: "feat x", Title: "t"},
		{Base: "main", Head: "feat-x", Title: ""},
		{Base: "main", Head: "../evil", Title: "t"},
	} {
		if err := pr.Validate(); err == nil {
			t.Fatalf("expected rejection for %#v, got none", pr)
		}
	}
}
