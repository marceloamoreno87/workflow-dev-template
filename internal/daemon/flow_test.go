package daemon

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSkeletonServesProjections(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(root)

	ready := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, cfg, &fakeSource{items: []IntakeItem{{ID: "owner/repo#7"}}}, ready)
	}()

	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		t.Fatalf("serve exited early: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("daemon never served")
	}
	get := func(path, token string) (int, string) {
		req, _ := http.NewRequest("GET", "http://"+addr+path, nil)
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
	if code, _ := get("/", ""); code != http.StatusUnauthorized {
		t.Fatalf("anonymous should be 401, got %d", code)
	}
	code, body := get("/", cfg.Token)
	if code != http.StatusOK {
		t.Fatalf("authed index should be 200, got %d", code)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if strings.Contains(body, "owner/repo#7") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("projection never served:\n%s", body)
		}
		time.Sleep(50 * time.Millisecond)
		code, body = get("/", cfg.Token)
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
}
