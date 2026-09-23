package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, workspace, body string) string {
	t.Helper()

	dir := t.TempDir()
	token := filepath.Join(dir, "token")
	if err := os.WriteFile(token, []byte("operator-token-at-least-16"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "daemon.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func validBody(workspace string) string {
	return "schemaVersion: harness.daemon/v1\nworkspace: " + workspace + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\n"
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg, err := LoadConfig(writeConfig(t, root, validBody(root)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspaceRoot != root || cfg.PollInterval != 30*time.Second || cfg.BindAddr != "127.0.0.1:8080" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestRejectBadConfigs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "bad schema", body: "schemaVersion: harness/v9\nworkspace: " + root + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\n"},
		{name: "unknown field", body: validBody(root) + "teleport: true\n"},
		{name: "relative workspace", body: validBody("ws")},
		{name: "bad interval", body: "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 1s\nbind: 127.0.0.1:8080\ntokenFile: token\n"},
		{name: "non-loopback bind", body: "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 30s\nbind: 0.0.0.0:8080\ntokenFile: token\n"},
		{name: "missing token", body: "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: absent\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadConfig(writeConfig(t, root, tc.body)); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestRejectShortToken(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "token")
	if err := os.WriteFile(token, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "daemon.yaml")
	body := "schemaVersion: harness.daemon/v1\nworkspace: " + dir + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected short-token rejection, got none")
	}
}
