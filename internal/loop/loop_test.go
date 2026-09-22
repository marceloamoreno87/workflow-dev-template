package loop

import (
	"testing"
	"time"
)

func beginFixture(t *testing.T) LoopState {
	t.Helper()

	s, err := Begin("owner/repo#123", []string{"product", "implementer", "reviewer"}, 10, 2*time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBeginAndSpecReady(t *testing.T) {
	t.Parallel()

	s := beginFixture(t)
	if s.Stage != StageSpecifying {
		t.Fatalf("expected specifying, got %q", s.Stage)
	}
	next, decision, err := Advance(s, LoopEvent{Kind: EventSpecReady}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Next != "implementer" || decision.Done || decision.Blocked {
		t.Fatalf("unexpected decision: %#v", decision)
	}
	if next.Stage != StageImplementing {
		t.Fatalf("expected implementing, got %q", next.Stage)
	}
}

func TestSpecOnlyWorkCompletes(t *testing.T) {
	t.Parallel()

	s, err := Begin("owner/repo#124", []string{"product"}, 10, 2*time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, decision, err := Advance(s, LoopEvent{Kind: EventSpecReady}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Done || decision.Next != "" {
		t.Fatalf("spec-only work should complete: %#v", decision)
	}
}

func TestRejectBadBegins(t *testing.T) {
	t.Parallel()

	now := time.Now()
	for _, tc := range []struct {
		name     string
		workItem string
		roles    []string
		budget   float64
		maxDur   time.Duration
	}{
		{name: "empty work item", workItem: "", roles: []string{"implementer"}, budget: 10, maxDur: time.Hour},
		{name: "no roles", workItem: "owner/repo#1", roles: nil, budget: 10, maxDur: time.Hour},
		{name: "bad role", workItem: "owner/repo#1", roles: []string{"evil role"}, budget: 10, maxDur: time.Hour},
		{name: "zero budget", workItem: "owner/repo#1", roles: []string{"implementer"}, budget: 0, maxDur: time.Hour},
		{name: "zero duration", workItem: "owner/repo#1", roles: []string{"implementer"}, budget: 10, maxDur: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Begin(tc.workItem, tc.roles, tc.budget, tc.maxDur, now); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestRejectWrongStageEvent(t *testing.T) {
	t.Parallel()

	s := beginFixture(t)
	if _, _, err := Advance(s, LoopEvent{Kind: EventGatesPassed}, time.Now()); err == nil {
		t.Fatal("expected stage rejection, got none")
	}
}
