package loop

import (
	"testing"
	"time"
)

func TestGatesPassedAdvancesToReviewer(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s := beginFixture(t)
	s, _, err := Advance(s, LoopEvent{Kind: EventSpecReady}, now)
	if err != nil {
		t.Fatal(err)
	}
	s, decision, err := Advance(s, LoopEvent{Kind: EventGatesPassed}, now)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Next != "reviewer" || s.Stage != StageReviewing {
		t.Fatalf("expected reviewer, got %#v in %q", decision, s.Stage)
	}
}

func TestReviewApprovalCompletes(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s := beginFixture(t)
	s, _, _ = Advance(s, LoopEvent{Kind: EventSpecReady}, now)
	s, _, _ = Advance(s, LoopEvent{Kind: EventGatesPassed}, now)
	s, decision, err := Advance(s, LoopEvent{Kind: EventReviewed, Verdict: VerdictApprove}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Done || s.Stage != StageDone {
		t.Fatalf("expected done, got %#v in %q", decision, s.Stage)
	}
	if _, _, err := Advance(s, LoopEvent{Kind: EventReviewed, Verdict: VerdictApprove}, now); err == nil {
		t.Fatal("expected terminal rejection, got none")
	}
}

func TestNoReviewerCompletesAfterGates(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s, err := Begin("owner/repo#125", []string{"product", "implementer"}, 10, 2*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	s, _, _ = Advance(s, LoopEvent{Kind: EventSpecReady}, now)
	_, decision, err := Advance(s, LoopEvent{Kind: EventGatesPassed}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Done {
		t.Fatalf("expected done without reviewer, got %#v", decision)
	}
}
