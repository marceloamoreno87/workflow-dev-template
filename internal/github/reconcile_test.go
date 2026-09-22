package github

import (
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func openIssue() IntakeIssue {
	issue, err := ParseIssue([]byte(`{"repository": "owner/repo", "number": 123, "title": "Fix login", "state": "open", "author": "octocat"}`))
	if err != nil {
		panic(err)
	}
	return issue
}

func itemIn(state workflow.State) workflow.WorkItem {
	return workflow.WorkItem{ID: "owner/repo#123", State: state, Version: 3}
}

func TestReconcileSubmitsNewWork(t *testing.T) {
	t.Parallel()

	res, err := Reconcile(workflow.WorkItem{}, openIssue(), workflow.StateInbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commands) != 1 || res.Commands[0].Type != workflow.CommandSubmitWork {
		t.Fatalf("unexpected commands: %#v", res.Commands)
	}
	if res.Commands[0].AggregateID != "owner/repo#123" || res.Commands[0].ExpectedVersion != 0 {
		t.Fatalf("unexpected submit: %#v", res.Commands[0])
	}
}

func TestReconcileAdvancesOneStep(t *testing.T) {
	t.Parallel()

	res, err := Reconcile(itemIn(workflow.StateInbox), openIssue(), workflow.StateTriage)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commands) != 1 || res.Commands[0].Type != workflow.CommandBeginTriage {
		t.Fatalf("unexpected commands: %#v", res.Commands)
	}
	if res.Commands[0].ExpectedVersion != 3 || res.IntentRecorded {
		t.Fatalf("unexpected reconcile: %#v", res.Commands[0])
	}
}

func TestReconcileNoOpinionIsNoop(t *testing.T) {
	t.Parallel()

	for _, status := range []workflow.State{"", workflow.StateImplementing} {
		res, err := Reconcile(itemIn(workflow.StateImplementing), openIssue(), status)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Commands) != 0 || res.IntentRecorded {
			t.Fatalf("expected noop, got %#v", res)
		}
	}
}

func TestReconcileRejectsJumps(t *testing.T) {
	t.Parallel()

	if _, err := Reconcile(itemIn(workflow.StateInbox), openIssue(), workflow.StateDone); err == nil {
		t.Fatal("expected jump rejection, got none")
	}
}

func TestReconciledCommandPassesWorkflow(t *testing.T) {
	t.Parallel()

	current := itemIn(workflow.StateInbox)
	res, err := Reconcile(current, openIssue(), workflow.StateTriage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (workflow.Workflow{}).Handle(time.Now(), current, res.Commands[0]); err != nil {
		t.Fatalf("reconciled command rejected by workflow: %v", err)
	}
}
