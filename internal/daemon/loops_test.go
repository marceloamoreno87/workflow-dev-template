package daemon

import (
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/loop"
)

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg, err := LoadConfig(writeConfig(t, root, validBody(root)))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RequiredRoles) != 3 || cfg.BudgetUSD != 10 || cfg.MaxDuration != 2*time.Hour {
		t.Fatalf("defaults missing: %#v", cfg)
	}
}

func TestConfigCustomLoopPolicy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	body := "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\nroles: [implementer]\nbudgetUSD: 5\nmaxDuration: 30m\n"
	cfg, err := LoadConfig(writeConfig(t, root, body))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RequiredRoles) != 1 || cfg.BudgetUSD != 5 || cfg.MaxDuration != 30*time.Minute {
		t.Fatalf("custom policy lost: %#v", cfg)
	}
}

func TestConfigRejectsBadPolicy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "no roles", body: "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\nroles: []\n"},
		{name: "zero budget", body: "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\nbudgetUSD: 0\n"},
		{name: "zero duration", body: "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\nmaxDuration: 0s\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadConfig(writeConfig(t, root, tc.body)); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestLoopRecordRoundTrip(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	rec := LoopRecord{Title: "First", ProjectID: "demo"}
	state, err := loop.Begin("owner/repo#1", []string{"product", "implementer", "reviewer"}, 10, 2*time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rec.Loop = state
	if err := d.saveLoop("owner/repo#1", rec); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	got, ok := reopened.LoopState("owner/repo#1")
	if !ok || got.Stage != loop.StageSpecifying || got.BudgetUSD != 10 {
		t.Fatalf("record lost: %#v %v", got, ok)
	}
}
