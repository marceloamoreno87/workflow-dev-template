package loop

import (
	"testing"
	"time"
)

func inReview(t *testing.T) LoopState {
	t.Helper()

	now := time.Now()
	s := beginFixture(t)
	s, _, _ = Advance(s, LoopEvent{Kind: EventSpecReady}, now)
	s, _, _ = Advance(s, LoopEvent{Kind: EventGatesPassed}, now)
	return s
}

func TestRepeatedFailureBlocks(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s := inReview(t)
	s, decision, err := Advance(s, LoopEvent{Kind: EventReviewed, Verdict: VerdictChanges, Signature: "gate-a"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Next != "implementer" {
		t.Fatalf("first novel failure should resume implementer: %#v", decision)
	}
	s, _, _ = Advance(s, LoopEvent{Kind: EventGatesPassed}, now)
	s, decision, err = Advance(s, LoopEvent{Kind: EventReviewed, Verdict: VerdictChanges, Signature: "gate-a"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Blocked || decision.Reason != "repeated failure" {
		t.Fatalf("same signature twice should block: %#v", decision)
	}
	if s.Stage != StageBlocked || s.BlockedReason != "repeated failure" {
		t.Fatalf("blocked state not recorded: %#v", s)
	}
}

func TestNovelFailureResetsStreak(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s := inReview(t)
	s, _, _ = Advance(s, LoopEvent{Kind: EventReviewed, Verdict: VerdictChanges, Signature: "gate-a"}, now)
	s, _, _ = Advance(s, LoopEvent{Kind: EventGatesPassed}, now)
	s, decision, err := Advance(s, LoopEvent{Kind: EventReviewed, Verdict: VerdictChanges, Signature: "gate-b"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Blocked || decision.Next != "implementer" {
		t.Fatalf("novel failure should resume, not block: %#v", decision)
	}
}

func TestFixBudgetExhausts(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s := beginFixture(t)
	s, _, _ = Advance(s, LoopEvent{Kind: EventSpecReady}, now)
	for i := 0; i < 3; i++ {
		var decision Decision
		var err error
		s, decision, err = Advance(s, LoopEvent{Kind: EventGatesFailed, Signature: string(rune('a' + i))}, now)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Blocked {
			t.Fatalf("fix %d should still resume: %#v", i+1, decision)
		}
	}
	s, decision, err := Advance(s, LoopEvent{Kind: EventGatesFailed, Signature: "d"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Blocked || decision.Reason != "fix budget exhausted" {
		t.Fatalf("fourth failure should block: %#v", decision)
	}
}

func TestPolicyAndScopeBlockImmediately(t *testing.T) {
	t.Parallel()

	now := time.Now()
	for _, verdict := range []Verdict{VerdictPolicy, VerdictScope} {
		s := inReview(t)
		_, decision, err := Advance(s, LoopEvent{Kind: EventReviewed, Verdict: verdict}, now)
		if err != nil {
			t.Fatal(err)
		}
		if !decision.Blocked {
			t.Fatalf("verdict %q should block: %#v", verdict, decision)
		}
	}
}

func TestBudgetAndDeadlineBlock(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s := beginFixture(t)
	if _, decision, err := Advance(s, LoopEvent{Kind: EventSpecReady, CostUSD: 11}, now); err != nil || !decision.Blocked {
		t.Fatalf("expected budget block, got %#v, %v", decision, err)
	}
	s = beginFixture(t)
	s.StartedAt = now.Add(-3 * time.Hour)
	if _, decision, err := Advance(s, LoopEvent{Kind: EventSpecReady}, now); err != nil || !decision.Blocked {
		t.Fatalf("expected deadline block, got %#v, %v", decision, err)
	}
}
