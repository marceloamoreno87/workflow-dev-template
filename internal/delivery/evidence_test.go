package delivery

import (
	"testing"
	"time"
)

func validEvidence() Evidence {
	start := time.Now()
	return Evidence{
		CommandID:     "cmd-1",
		Tool:          "go-test",
		Version:       "1.27.1",
		StartedAt:     start,
		FinishedAt:    start.Add(time.Minute),
		ExitCode:      0,
		Commit:        "abcdef",
		Applicable:    true,
		Deterministic: true,
	}
}

func TestValidateEvidence(t *testing.T) {
	t.Parallel()

	if err := validEvidence().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNotApplicableNeedsReason(t *testing.T) {
	t.Parallel()

	e := validEvidence()
	e.Applicable = false
	if err := e.Validate(); err == nil {
		t.Fatal("expected reason requirement, got none")
	}
	e.Note = "no E2E for docs-only change"
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadEvidence(t *testing.T) {
	t.Parallel()

	mk := func(mut func(*Evidence)) Evidence {
		e := validEvidence()
		mut(&e)
		return e
	}
	for _, tc := range []struct {
		name string
		e    Evidence
	}{
		{name: "empty command", e: mk(func(e *Evidence) { e.CommandID = "" })},
		{name: "empty tool", e: mk(func(e *Evidence) { e.Tool = "" })},
		{name: "empty version", e: mk(func(e *Evidence) { e.Version = "" })},
		{name: "inverted times", e: mk(func(e *Evidence) { e.StartedAt, e.FinishedAt = e.FinishedAt, e.StartedAt })},
		{name: "zero times", e: mk(func(e *Evidence) { e.StartedAt = time.Time{} })},
		{name: "negative exit", e: mk(func(e *Evidence) { e.ExitCode = -1 })},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.e.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
