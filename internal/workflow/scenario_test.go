package workflow_test

import (
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/gatekeeper"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestHappyPathReplay(t *testing.T) {
	t.Parallel()

	w := workflow.Workflow{}
	p := gatekeeper.Policy{}
	profile := gatekeeper.ProfileStandard
	actorKind := gatekeeper.ActorOperator
	actorID := workflow.ActorID("operator-1")
	aggregateID := workflow.WorkItemID("repo#1")

	sequence := []workflow.CommandType{
		workflow.CommandSubmitWork,
		workflow.CommandBeginTriage,
		workflow.CommandAuthorizeWork,
		workflow.CommandBeginSpec,
		workflow.CommandApproveSpec,
		workflow.CommandBeginImplementation,
		workflow.CommandSubmitReview,
		workflow.CommandApprovePR,
		workflow.CommandBeginDeploy,
		workflow.CommandMarkDeploymentHealthy,
		workflow.CommandAcceptFeature,
		workflow.CommandCompleteRollout,
	}

	var history []workflow.Event
	item := workflow.WorkItem{}
	now := time.Unix(1, 0)

	for i, commandType := range sequence {
		command := workflow.Command{
			ID:              workflow.CommandID(string(rune('a'+i)) + "-cmd"),
			AggregateID:     aggregateID,
			ExpectedVersion: item.Version,
			ActorID:         actorID,
			Type:            commandType,
		}
		if i == 0 {
			// SubmitWork against zero-value WorkItem: Gatekeeper state is empty.
			decision := p.Decide(gatekeeper.Context{Actor: actorKind, Profile: profile}, command)
			if !decision.Allowed {
				t.Fatalf("gatekeeper denied SubmitWork: %#v", decision)
			}
		} else {
			decision := p.Decide(gatekeeper.Context{State: item.State, Actor: actorKind, Profile: profile}, command)
			if !decision.Allowed {
				t.Fatalf("gatekeeper denied %q in state %q: %#v", commandType, item.State, decision)
			}
		}

		events, err := w.Handle(now, item, command)
		if err != nil {
			t.Fatalf("Handle(%q) error = %v", commandType, err)
		}
		if len(events) == 0 {
			t.Fatalf("Handle(%q) returned no events", commandType)
		}
		history = append(history, events...)

		folded, err := workflow.Fold(history)
		if err != nil {
			t.Fatalf("Fold after %q error = %v", commandType, err)
		}
		item = folded
		now = now.Add(time.Second)
	}

	if item.State != workflow.StateDone {
		t.Fatalf("final State = %q, want %q", item.State, workflow.StateDone)
	}
	if int(item.Version) != len(history) {
		t.Fatalf("final Version = %d, want event count %d", item.Version, len(history))
	}

	replayed, err := workflow.Fold(append([]workflow.Event(nil), history...))
	if err != nil {
		t.Fatalf("replay Fold error = %v", err)
	}
	if replayed != item {
		t.Fatalf("replay mismatch: got %#v, want %#v", replayed, item)
	}
}
