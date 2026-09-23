package daemon

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestDashboardPostCommits(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Tick(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(d.Dashboard().Handler())
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	req, _ := http.NewRequest("POST", server.URL+"/api/commands", strings.NewReader(`{"aggregateId":"owner/repo#1","expectedVersion":2,"type":"authorize_work"}`))
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://"+host)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	if doc["version"] != float64(3) {
		t.Fatalf("unexpected echo: %v", doc)
	}
	if got := mustLoad(t, d, "owner/repo#1"); got.State != workflow.StateReady {
		t.Fatalf("journal not moved: %#v", got)
	}
}

func TestTelegramBatchFlow(t *testing.T) {
	now := time.Now()
	msg := func(updateID, from int64, text string, at int64) string {
		return `{"update_id":` + itoa(updateID) + `,"message":{"message_id":1,"from":{"id":` + itoa(from) + `},"chat":{"id":2002,"type":"private"},"text":` + quote(text) + `,"date":` + itoa(at) + `}}`
	}
	updates := strings.Join([]string{
		msg(51, 1001, "/changes owner/repo#1 5 needs tests", now.Unix()),
		msg(52, 1001, "/changes owner/repo#1 5 stale news", now.Add(-time.Hour).Unix()),
		msg(53, 9999, "/changes owner/repo#1 5 intruder", now.Unix()),
		msg(54, 1001, "/approve owner/repo#1 5", now.Unix()),
	}, ",")
	var sent []string
	server := telegramDouble(t, updates, &sent)
	defer server.Close()

	d := telegramDaemon(t, server)
	seedReviewing(t, d, "owner/repo#1")
	if err := d.pollTelegram(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	got := mustLoad(t, d, "owner/repo#1")
	if got.State != workflow.StateChangesRequested || got.Version != 6 {
		t.Fatalf("plain command not applied exactly once: %#v", got)
	}
	if len(sent) != 1 || !strings.Contains(sent[0], "challenge") {
		t.Fatalf("sensitive command mishandled: %v", sent)
	}
	raw, err := os.ReadFile(filepath.Join(d.cfg.WorkspaceRoot, ".harness", "telegram-offset"))
	if err != nil || strings.TrimSpace(string(raw)) != "55" {
		t.Fatalf("offset not advanced past batch: %q %v", raw, err)
	}

	// Restart against an empty double: versions frozen, nothing redelivered.
	empty := telegramDouble(t, "", &sent)
	defer empty.Close()
	d.cfg.Telegram.BaseURL = empty.URL
	quiet, err := d.Tick(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if quiet {
		t.Fatal("second pass must be quiet")
	}
	if again := mustLoad(t, d, "owner/repo#1"); again.Version != 6 {
		t.Fatalf("duplicate apply after restart: %#v", again)
	}
}

func quote(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}
