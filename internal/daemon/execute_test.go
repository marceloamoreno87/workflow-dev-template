package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/loop"
	"github.com/marceloamoreno87/workflow-dev-template/internal/registry"
)

func setupPipeline(t *testing.T, goStub string) (string, string, *Daemon) {
	t.Helper()

	root := fixtureWorkspace(t)
	bin := stubBin(t)
	writeStub(t, bin, "gofmt", "#!/bin/sh\nexit 0\n")
	writeStub(t, bin, "go", goStub)
	d, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return root, bin, d
}

func rewriteMode(t *testing.T, bin, mode string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(bin, "mode"), []byte(mode), 0o644); err != nil {
		t.Fatal(err)
	}
}

func resolveFixture(t *testing.T, d *Daemon) (string, string, registry.ProjectManifest) {
	t.Helper()

	projectID, sub, manifest, found, err := d.resolveProject("owner/repo#123")
	if err != nil || !found {
		t.Fatalf("fixture not resolved: %v %v", found, err)
	}
	return projectID, sub, manifest
}

func baseRecord(t *testing.T) LoopRecord {
	t.Helper()

	state, err := loop.Begin("owner/repo#123", []string{"product", "implementer", "reviewer"}, 10, 2*time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return LoopRecord{Loop: state, Title: "Add health endpoint", ProjectID: "demo"}
}

func TestImplementerPassesGates(t *testing.T) {
	_, _, d := setupPipeline(t, "#!/bin/sh\nexit 0\n")
	projectID, sub, manifest := resolveFixture(t, d)

	outcome, err := d.ExecuteRole(context.Background(), "owner/repo#123", baseRecord(t), projectID, sub, manifest, "implementer", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Approved || outcome.FailureSignature != "" {
		t.Fatalf("expected pass: %#v", outcome)
	}
	if len(outcome.Bundle.Evidence) != 3 {
		t.Fatalf("expected 3 gate evidences (format, lint, test): %#v", outcome.Bundle)
	}
	for _, e := range outcome.Bundle.Evidence {
		if err := e.Validate(); err != nil {
			t.Fatalf("evidence invalid: %v %#v", err, e)
		}
	}
	if outcome.Bundle.Usage.Model != "gpt-5.6-terra" || outcome.Bundle.Usage.InputTokens != 10 {
		t.Fatalf("usage lost: %#v", outcome.Bundle.Usage)
	}
}

func TestImplementerGateFailure(t *testing.T) {
	_, _, d := setupPipeline(t, "#!/bin/sh\ncase \"$*\" in *test*) exit 1;; *) exit 0;; esac\n")
	projectID, sub, manifest := resolveFixture(t, d)

	outcome, err := d.ExecuteRole(context.Background(), "owner/repo#123", baseRecord(t), projectID, sub, manifest, "implementer", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Approved || outcome.FailureSignature != "test" {
		t.Fatalf("expected test-gate failure signature: %#v", outcome)
	}
	if len(outcome.Bundle.Evidence) != 3 {
		t.Fatalf("all gates must leave evidence: %#v", outcome.Bundle)
	}
}

func TestReviewerMapsBlockedToChanges(t *testing.T) {
	_, bin, d := setupPipeline(t, "#!/bin/sh\nexit 0\n")
	projectID, sub, manifest := resolveFixture(t, d)

	rewriteMode(t, bin, "blocked")

	outcome, err := d.ExecuteRole(context.Background(), "owner/repo#123", baseRecord(t), projectID, sub, manifest, "reviewer", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Approved || outcome.FailureSignature != "review:blocked" {
		t.Fatalf("expected changes mapping: %#v", outcome)
	}
}

func TestTerminalClosesWorktree(t *testing.T) {
	root, _, d := setupPipeline(t, "#!/bin/sh\nexit 0\n")

	prepared, err := d.ensureWorktree("demo", filepath.Join(root, "projects", "demo"))
	if err != nil {
		t.Fatal(err)
	}
	rec := baseRecord(t)
	rec.Loop = loop.LoopState{WorkItem: "owner/repo#123", Stage: loop.StageDone}
	if err := d.saveLoop("owner/repo#123", rec); err != nil {
		t.Fatal(err)
	}
	seedReady(t, d, "owner/repo#123")

	didWork, err := d.advanceItem(context.Background(), "owner/repo#123", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !didWork {
		t.Fatal("expected close work")
	}
	if _, err := os.Stat(prepared.Dir); !os.IsNotExist(err) {
		t.Fatalf("worktree survives terminal: %v", err)
	}
	if _, err := d.advanceItem(context.Background(), "owner/repo#123", time.Now()); err != nil {
		t.Fatalf("second close must be a no-op: %v", err)
	}
}
