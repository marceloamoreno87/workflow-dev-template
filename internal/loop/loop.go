// internal/loop/loop.go
package loop

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

var ErrLoop = errors.New("invalid role loop transition")

type Stage string

const (
	StageSpecifying   Stage = "specifying"
	StageImplementing Stage = "implementing"
	StageReviewing    Stage = "reviewing"
	StageDone         Stage = "done"
	StageBlocked      Stage = "blocked"
)

type EventKind string

const (
	EventSpecReady   EventKind = "spec_ready"
	EventGatesPassed EventKind = "gates_passed"
	EventGatesFailed EventKind = "gates_failed"
	EventReviewed    EventKind = "reviewed"
)

type Verdict string

const (
	VerdictApprove Verdict = "approve"
	VerdictChanges Verdict = "changes"
	VerdictPolicy  Verdict = "policy"
	VerdictScope   Verdict = "scope"
)

type LoopEvent struct {
	Kind      EventKind
	Signature string
	Verdict   Verdict
	CostUSD   float64
}

type Decision struct {
	Next    string
	Done    bool
	Blocked bool
	Reason  string
}

type LoopState struct {
	WorkItem      string
	Stage         Stage
	RequiredRoles []string
	Fixes         int
	LastSignature string
	SameStreak    int
	SpentUSD      float64
	BudgetUSD     float64
	StartedAt     time.Time
	MaxDuration   time.Duration
	BlockedReason string
}

const maxFixes = 3

var rolePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

var pipeline = []string{"implementer", "reviewer"}

func Begin(workItem string, roles []string, budgetUSD float64, maxDuration time.Duration, now time.Time) (LoopState, error) {
	if workItem == "" {
		return LoopState{}, fmt.Errorf("%w: work item is required", ErrLoop)
	}
	if len(roles) == 0 {
		return LoopState{}, fmt.Errorf("%w: at least one required role", ErrLoop)
	}
	for _, role := range roles {
		if !rolePattern.MatchString(role) {
			return LoopState{}, fmt.Errorf("%w: role %q", ErrLoop, role)
		}
	}
	if !(budgetUSD > 0) {
		return LoopState{}, fmt.Errorf("%w: budget must be positive", ErrLoop)
	}
	if maxDuration <= 0 {
		return LoopState{}, fmt.Errorf("%w: max duration must be positive", ErrLoop)
	}
	return LoopState{
		WorkItem:      workItem,
		Stage:         StageSpecifying,
		RequiredRoles: append([]string{}, roles...),
		BudgetUSD:     budgetUSD,
		StartedAt:     now,
		MaxDuration:   maxDuration,
	}, nil
}

func required(set []string, role string) bool {
	for _, r := range set {
		if r == role {
			return true
		}
	}
	return false
}

func nextPipelineRole(set []string, after string) string {
	start := 0
	for i, role := range pipeline {
		if role == after {
			start = i + 1
			break
		}
	}
	for _, role := range pipeline[start:] {
		if required(set, role) {
			return role
		}
	}
	return ""
}

func block(s LoopState, reason string) (LoopState, Decision, error) {
	s.Stage = StageBlocked
	s.BlockedReason = reason
	return s, Decision{Blocked: true, Reason: reason}, nil
}

func Advance(s LoopState, e LoopEvent, now time.Time) (LoopState, Decision, error) {
	if s.Stage == StageDone || s.Stage == StageBlocked {
		return LoopState{}, Decision{}, fmt.Errorf("%w: loop is terminal", ErrLoop)
	}
	if e.CostUSD < 0 {
		return LoopState{}, Decision{}, fmt.Errorf("%w: negative cost", ErrLoop)
	}
	if s.SpentUSD+e.CostUSD > s.BudgetUSD {
		return block(s, "budget breach")
	}
	if !now.Before(s.StartedAt.Add(s.MaxDuration)) {
		return block(s, "deadline exceeded")
	}
	s.SpentUSD += e.CostUSD

	switch s.Stage {
	case StageSpecifying:
		if e.Kind != EventSpecReady {
			return LoopState{}, Decision{}, fmt.Errorf("%w: specifying accepts only spec_ready", ErrLoop)
		}
		if next := nextPipelineRole(s.RequiredRoles, ""); next != "" {
			s.Stage = StageImplementing
			return s, Decision{Next: next, Reason: "spec complete"}, nil
		}
		s.Stage = StageDone
		return s, Decision{Done: true, Reason: "spec-only work complete"}, nil
	case StageImplementing:
		switch e.Kind {
		case EventGatesPassed:
			if next := nextPipelineRole(s.RequiredRoles, "implementer"); next != "" {
				s.Stage = StageReviewing
				return s, Decision{Next: next, Reason: "gates passed"}, nil
			}
			s.Stage = StageDone
			return s, Decision{Done: true, Reason: "gates passed, no reviewer required"}, nil
		case EventGatesFailed:
			return recordFailure(s, e.Signature, "gates failed")
		default:
			return LoopState{}, Decision{}, fmt.Errorf("%w: implementing accepts only gates events", ErrLoop)
		}
	case StageReviewing:
		if e.Kind != EventReviewed {
			return LoopState{}, Decision{}, fmt.Errorf("%w: reviewing accepts only reviewed", ErrLoop)
		}
		switch e.Verdict {
		case VerdictApprove:
			s.Stage = StageDone
			return s, Decision{Done: true, Reason: "review approved"}, nil
		case VerdictChanges:
			return recordFailure(s, e.Signature, "changes requested")
		case VerdictPolicy:
			return block(s, "policy conflict")
		case VerdictScope:
			return block(s, "scope expansion")
		default:
			return LoopState{}, Decision{}, fmt.Errorf("%w: unknown verdict", ErrLoop)
		}
	default:
		return LoopState{}, Decision{}, fmt.Errorf("%w: unknown stage", ErrLoop)
	}
}

func recordFailure(s LoopState, signature, reason string) (LoopState, Decision, error) {
	if s.Fixes >= maxFixes {
		return block(s, "fix budget exhausted")
	}
	s.Fixes++
	if signature != "" && signature == s.LastSignature {
		s.SameStreak++
	} else {
		s.SameStreak = 1
		s.LastSignature = signature
	}
	if s.SameStreak >= 2 {
		return block(s, "repeated failure")
	}
	s.Stage = StageImplementing
	return s, Decision{Next: "implementer", Reason: reason}, nil
}
