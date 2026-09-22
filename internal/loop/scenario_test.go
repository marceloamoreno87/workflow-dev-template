package loop

import (
	"testing"
	"time"
)

func TestHappyPathLoop(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s, err := Begin("owner/repo#200", []string{"product", "implementer", "reviewer"}, 10, 2*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		event LoopEvent
		next  string
		done  bool
	}{
		{event: LoopEvent{Kind: EventSpecReady, CostUSD: 1}, next: "implementer"},
		{event: LoopEvent{Kind: EventGatesFailed, Signature: "lint", CostUSD: 2}, next: "implementer"},
		{event: LoopEvent{Kind: EventGatesPassed, CostUSD: 2}, next: "reviewer"},
		{event: LoopEvent{Kind: EventReviewed, Verdict: VerdictApprove, CostUSD: 1}, done: true},
	}
	var decision Decision
	for i, step := range steps {
		s, decision, err = Advance(s, step.event, now)
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if decision.Next != step.next || decision.Done != step.done {
			t.Fatalf("step %d: got %#v", i, decision)
		}
	}
	if s.Stage != StageDone || s.SpentUSD != 6 {
		t.Fatalf("unexpected final state: %#v", s)
	}
}

func TestBlockedLoopRecordsReason(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s, err := Begin("owner/repo#201", []string{"product", "implementer", "reviewer"}, 10, 2*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	s, _, _ = Advance(s, LoopEvent{Kind: EventSpecReady}, now)
	s, _, _ = Advance(s, LoopEvent{Kind: EventGatesFailed, Signature: "lint"}, now)
	s, _, _ = Advance(s, LoopEvent{Kind: EventGatesFailed, Signature: "test"}, now)
	s, _, _ = Advance(s, LoopEvent{Kind: EventGatesFailed, Signature: "build"}, now)
	s, decision, err := Advance(s, LoopEvent{Kind: EventGatesFailed, Signature: "vet"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Blocked || s.BlockedReason != "fix budget exhausted" {
		t.Fatalf("expected exhausted block, got %#v in %#v", decision, s)
	}
	if _, _, err := Advance(s, LoopEvent{Kind: EventGatesPassed}, now); err == nil {
		t.Fatal("blocked loop must reject further events")
	}
}
