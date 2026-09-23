package daemon

import (
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

// seedReady drives an item to ready through direct journal applies, simulating
// what human surfaces (Telegram/dashboard) do through the same journal.
func seedReady(t *testing.T, d *Daemon, id workflow.WorkItemID) {
	t.Helper()

	now := time.Now()
	steps := []workflow.Command{
		{ID: "seed-submit", AggregateID: id, ExpectedVersion: 0, ActorID: "actor/automation", Type: workflow.CommandSubmitWork},
		{ID: "seed-triage", AggregateID: id, ExpectedVersion: 1, ActorID: "actor/automation", Type: workflow.CommandBeginTriage},
		{ID: "seed-authorize", AggregateID: id, ExpectedVersion: 2, ActorID: "actor/operator", Type: workflow.CommandAuthorizeWork},
	}
	for _, cmd := range steps {
		if _, err := d.Journal().Apply(now, cmd); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAutoTriage(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	didWork, err := d.Tick(ctx(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !didWork {
		t.Fatal("expected intake plus triage, got none")
	}
	item := mustLoad(t, d, "owner/repo#1")
	if item.State != workflow.StateTriage || item.Version != 2 {
		t.Fatalf("expected triage at version 2: %#v", item)
	}
}

func mustLoad(t *testing.T, d *Daemon, id workflow.WorkItemID) workflow.WorkItem {
	t.Helper()

	item, err := d.Journal().Load(id)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func TestTriageWaitsForHuman(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Tick(ctx(), time.Now()); err != nil {
		t.Fatal(err)
	}
	quiet, err := d.Tick(ctx(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if quiet {
		t.Fatal("triaged item must wait for the human gate")
	}
}

func TestUnregisteredSkips(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	seedReady(t, d, "owner/ghost#7")
	didWork, err := d.Tick(ctx(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if didWork {
		t.Fatal("unregistered repository must skip without work")
	}
	item := mustLoad(t, d, "owner/ghost#7")
	if item.State != workflow.StateReady {
		t.Fatalf("unregistered item disturbed: %#v", item)
	}
}
