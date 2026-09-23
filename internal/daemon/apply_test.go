package daemon

import (
	"errors"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/dashboard"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func isConflict(err error) bool {
	return errors.Is(err, dashboard.ErrApplyConflict)
}

func TestApplyOperatorCommand(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Tick(ctx(), time.Now()); err != nil {
		t.Fatal(err)
	}
	version, err := d.ApplyOperatorCommand(dashboard.CommandRequest{
		AggregateID: "owner/repo#1", ExpectedVersion: 2, Type: workflow.CommandAuthorizeWork,
	})
	if err != nil {
		t.Fatal(err)
	}
	if version != 3 {
		t.Fatalf("expected version 3, got %d", version)
	}
	if got := mustLoad(t, d, "owner/repo#1"); got.State != workflow.StateReady {
		t.Fatalf("not authorized: %#v", got)
	}
}

func TestApplyConflictListsAllowed(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	seedReady(t, d, "owner/repo#9")
	_, err = d.ApplyOperatorCommand(dashboard.CommandRequest{
		AggregateID: "owner/repo#9", ExpectedVersion: 0, Type: workflow.CommandBeginSpec,
	})
	if err == nil {
		t.Fatal("expected stale rejection, got none")
	}
	if !isConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestApplyRejectsDenied(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	seedReady(t, d, "owner/repo#9")
	// begin_triage is illegal from ready: the state machine must refuse it.
	if _, err := d.ApplyOperatorCommand(dashboard.CommandRequest{
		AggregateID: "owner/repo#9", ExpectedVersion: 3, Type: workflow.CommandBeginTriage,
	}); err == nil {
		t.Fatal("expected transition rejection, got none")
	}
}

func TestTelegramConfigDefaultsDisabled(t *testing.T) {
	root := t.TempDir()
	cfg, err := LoadConfig(writeConfig(t, root, validBody(root)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Telegram != nil {
		t.Fatal("telegram should default disabled")
	}
}
