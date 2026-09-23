package daemon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/githublive"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestAcceptanceScenario(t *testing.T) {
	// No t.Parallel: canaries ride process environment.
	t.Setenv("GH_TOKEN", "canary-gh-token-001")
	t.Setenv("GITHUB_TOKEN", "canary-github-token-002")
	t.Setenv("COOLIFY_TOKEN", "canary-coolify-token-003")
	t.Setenv("HARNESS_CANARY", "canary-control-004")
	canaries := []string{"canary-gh-token-001", "canary-github-token-002", "canary-coolify-token-003", "canary-control-004"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":78,"title":"Add health endpoint","body":"x","state":"open","user":{"login":"octocat"},"updated_at":"2026-09-23T00:00:00Z"}]`))
	}))
	defer server.Close()

	root := fixtureWorkspace(t)
	bin := stubBin(t)
	writeStub(t, bin, "gofmt", "#!/bin/sh\nexit 0\n")
	writeStub(t, bin, "go", "#!/bin/sh\nenv | sort > \"$(dirname \"$0\")/env-go.txt\"\nexit 0\n")

	client, err := githublive.NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := githublive.NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(testConfig(root), &pollerAdapter{poller: poller, owner: "owner", repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	authorizeAsHuman(t, d, "owner/repo#78", 2)
	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := mustLoad(t, d, "owner/repo#78"); got.State != workflow.StateReviewing || got.Version != 5 {
		t.Fatalf("implement leg incomplete: %#v", got)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(testConfig(root), &pollerAdapter{poller: poller, owner: "owner", repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	if _, err := reopened.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	final := mustLoad(t, reopened, "owner/repo#78")
	if final.State != workflow.StateReviewing || final.Version != 5 {
		t.Fatalf("restart broke the chain: %#v", final)
	}
	done, ok := reopened.LoopState("owner/repo#78")
	if !ok || done.Stage != "done" {
		t.Fatalf("loop not done: %#v %v", done, ok)
	}

	// Canary sweep over every artifact the run produced.
	artifacts := map[string]string{}
	dbBytes, err := os.ReadFile(DBPath(testConfig(root)))
	if err != nil {
		t.Fatal(err)
	}
	artifacts["harness.db"] = string(dbBytes)
	for _, name := range []string{"daemon-known.json", "daemon-loops.json"} {
		raw, err := os.ReadFile(filepath.Join(root, ".harness", name))
		if err != nil {
			t.Fatal(err)
		}
		artifacts[name] = string(raw)
	}
	for _, name := range []string{"argv.log", "stdin.txt", "env-codex.txt", "env-go.txt"} {
		raw, err := os.ReadFile(filepath.Join(bin, name))
		if err != nil {
			t.Fatal(err)
		}
		artifacts[name] = string(raw)
	}
	dash := httptest.NewServer(reopened.Dashboard().Handler())
	defer dash.Close()
	req, _ := http.NewRequest("GET", dash.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	artifacts["dashboard"] = string(body)
	for name, content := range artifacts {
		for _, canary := range canaries {
			if strings.Contains(content, canary) {
				t.Fatalf("canary %q leaked into %s", canary, name)
			}
		}
	}
}
