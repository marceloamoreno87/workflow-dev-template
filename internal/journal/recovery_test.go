package journal_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/gatekeeper"
	"github.com/marceloamoreno87/workflow-dev-template/internal/journal"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestRestartRecoversHappyPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "journal.db")
	s, err := journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	p := gatekeeper.Policy{}
	profile := gatekeeper.ProfileStandard
	sequence := []workflow.CommandType{
		workflow.CommandSubmitWork, workflow.CommandBeginTriage, workflow.CommandAuthorizeWork,
		workflow.CommandBeginSpec, workflow.CommandApproveSpec, workflow.CommandBeginImplementation,
		workflow.CommandSubmitReview, workflow.CommandApprovePR, workflow.CommandBeginDeploy,
		workflow.CommandMarkDeploymentHealthy, workflow.CommandAcceptFeature, workflow.CommandCompleteRollout,
	}

	now := time.Unix(1, 0).UTC()
	item := workflow.WorkItem{}
	for i, commandType := range sequence {
		cmd := workflow.Command{ID: workflow.CommandID(fmt.Sprintf("c%d", i+1)), AggregateID: "repo#1", ExpectedVersion: item.Version, ActorID: "operator", Type: commandType}
		state := item.State
		if i == 0 {
			if d := p.Decide(gatekeeper.Context{Actor: gatekeeper.ActorOperator, Profile: profile}, cmd); !d.Allowed {
				t.Fatalf("gatekeeper denied SubmitWork: %#v", d)
			}
		} else if d := p.Decide(gatekeeper.Context{State: state, Actor: gatekeeper.ActorOperator, Profile: profile}, cmd); !d.Allowed {
			t.Fatalf("gatekeeper denied %q: %#v", commandType, d)
		}
		events, err := s.Apply(now, cmd)
		if err != nil {
			t.Fatalf("Apply(%q) error = %v", commandType, err)
		}
		if len(events) == 0 {
			t.Fatalf("Apply(%q) returned no events", commandType)
		}
		loaded, err := s.Load("repo#1")
		if err != nil {
			t.Fatal(err)
		}
		item = loaded
		now = now.Add(time.Second)
	}
	before, err := s.Load("repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if before.State != workflow.StateDone || before.Version != workflow.Version(len(sequence)) {
		t.Fatalf("before restart = %#v", before)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	after, err := reopened.Load("repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("after restart = %#v, want %#v", after, before)
	}

	stale := workflow.Command{ID: "stale", AggregateID: "repo#1", ExpectedVersion: 1, ActorID: "operator", Type: workflow.CommandBeginTriage}
	if _, err := reopened.Apply(now, stale); err == nil {
		t.Fatal("stale Apply expected error")
	}
	unchanged, err := reopened.Load("repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged != after {
		t.Fatalf("stale command mutated state: %#v vs %#v", unchanged, after)
	}
}
