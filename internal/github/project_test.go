package github

import (
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestParseProjectStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   string
		want workflow.State
	}{
		{in: "", want: ""},
		{in: "Inbox", want: workflow.StateInbox},
		{in: "triage", want: workflow.StateTriage},
		{in: "Changes Requested", want: workflow.StateChangesRequested},
		{in: "ready_to_deploy", want: workflow.StateReadyToDeploy},
		{in: "Client QA", want: workflow.StateClientQA},
		{in: "rolling-out", want: workflow.StateRollingOut},
		{in: "DONE", want: workflow.StateDone},
	} {
		got, err := ParseProjectStatus(tc.in)
		if err != nil {
			t.Fatalf("input %q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("input %q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRejectBadProjectStatus(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"Backlog", "In Progress", "Shipped it"} {
		if _, err := ParseProjectStatus(in); err == nil {
			t.Fatalf("expected rejection for %q", in)
		}
	}
	if got, err := ParseProjectStatus("   "); err != nil || got != "" {
		t.Fatalf("whitespace is no opinion, got %q, %v", got, err)
	}
}
