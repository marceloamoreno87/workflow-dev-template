package daemon

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/journal"
)

func TestRunServesAndShutsDown(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(root)

	ready := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, cfg, &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}}, ready)
	}()

	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		t.Fatalf("serve exited early: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("daemon never served")
	}
	get := func(token string) (int, string) {
		req, _ := http.NewRequest("GET", "http://"+addr+"/", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(raw)
	}
	if code, _ := get(""); code != http.StatusUnauthorized {
		t.Fatalf("anonymous should be 401, got %d", code)
	}
	if code, _ := get(cfg.Token); code != http.StatusOK {
		t.Fatalf("authed index should be 200, got %d", code)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown hung")
	}
	db, err := journal.Open(DBPath(cfg))
	if err != nil {
		t.Fatalf("journal did not close cleanly: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(DBPath(cfg), ".harness/harness.db") {
		t.Fatalf("unexpected db path: %s", DBPath(cfg))
	}
}
