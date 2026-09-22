package github

import (
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func closedIssue() IntakeIssue {
	issue, err := ParseIssue([]byte(`{"repository": "owner/repo", "number": 123, "title": "Fix login", "state": "closed", "author": "octocat"}`))
	if err != nil {
		panic(err)
	}
	return issue
}

func TestClosedIssueRecordsIntentWithoutCommands(t *testing.T) {
	t.Parallel()

	for _, state := range []workflow.State{workflow.StateClientQA, workflow.StateReadyToDeploy, workflow.StateImplementing} {
		res, err := Reconcile(itemIn(state), closedIssue(), workflow.StateDone)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Commands) != 0 || !res.IntentRecorded {
			t.Fatalf("state %q: expected intent without commands, got %#v", state, res)
		}
	}
}

func TestClosedIssueWithoutWorkIsNoop(t *testing.T) {
	t.Parallel()

	res, err := Reconcile(workflow.WorkItem{}, closedIssue(), workflow.StateDone)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commands) != 0 || res.IntentRecorded {
		t.Fatalf("expected noop, got %#v", res)
	}
}

func TestBlockedNeverJumpsByIntake(t *testing.T) {
	t.Parallel()

	if _, err := Reconcile(itemIn(workflow.StateBlocked), openIssue(), workflow.StateReady); err == nil {
		t.Fatal("expected blocked jump rejection, got none")
	}
}

func TestTerminalNeverResurrectsByIntake(t *testing.T) {
	t.Parallel()

	for _, state := range []workflow.State{workflow.StateDone, workflow.StateCancelled} {
		if _, err := Reconcile(itemIn(state), openIssue(), workflow.StateInbox); err == nil {
			t.Fatalf("state %q: expected terminal rejection", state)
		}
		res, err := Reconcile(itemIn(state), closedIssue(), workflow.StateDone)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Commands) != 0 {
			t.Fatalf("terminal must emit no commands, got %#v", res)
		}
	}
}

func TestIntakeCommandIDsAreStable(t *testing.T) {
	t.Parallel()

	first, err := Reconcile(itemIn(workflow.StateInbox), openIssue(), workflow.StateTriage)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Reconcile(itemIn(workflow.StateInbox), openIssue(), workflow.StateTriage)
	if err != nil {
		t.Fatal(err)
	}
	if first.Commands[0].ID != second.Commands[0].ID {
		t.Fatalf("unstable command id: %q vs %q", first.Commands[0].ID, second.Commands[0].ID)
	}
}
