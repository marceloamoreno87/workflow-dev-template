package workflow

import (
	"errors"
	"testing"
	"time"
)

func TestFoldRebuildsStateAndVersion(t *testing.T) {
	t.Parallel()

	events := []Event{
		{ID: "e1", AggregateID: "repo#1", Version: 1, Type: EventWorkSubmitted, To: StateInbox, At: time.Unix(1, 0)},
		{ID: "e2", AggregateID: "repo#1", Version: 2, Type: EventStateChanged, From: StateInbox, To: StateTriage, At: time.Unix(2, 0)},
	}

	got, err := Fold(events)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "repo#1" || got.State != StateTriage || got.Version != 2 {
		t.Fatalf("unexpected aggregate: %#v", got)
	}
}

func TestFoldRejectsInvalidHistories(t *testing.T) {
	t.Parallel()

	validSubmitted := Event{ID: "e1", AggregateID: "repo#1", Version: 1, Type: EventWorkSubmitted, To: StateInbox, At: time.Unix(1, 0)}

	tests := []struct {
		name   string
		events []Event
		want   error
	}{
		{name: "empty history", want: ErrEmptyHistory},
		{name: "first event is not submission", events: []Event{{AggregateID: "repo#1", Version: 1, Type: EventStateChanged, To: StateInbox}}, want: ErrTransitionMismatch},
		{name: "first event has invalid aggregate", events: []Event{{Version: 1, Type: EventWorkSubmitted, To: StateInbox}}, want: ErrAggregateMismatch},
		{name: "first event has wrong version", events: []Event{{AggregateID: "repo#1", Version: 2, Type: EventWorkSubmitted, To: StateInbox}}, want: ErrVersionGap},
		{name: "first event has invalid destination", events: []Event{{AggregateID: "repo#1", Version: 1, Type: EventWorkSubmitted, To: State("invalid")}}, want: ErrTransitionMismatch},
		{name: "aggregate mismatch", events: []Event{validSubmitted, {AggregateID: "repo#2", Version: 2, Type: EventStateChanged, From: StateInbox, To: StateTriage}}, want: ErrAggregateMismatch},
		{name: "version gap", events: []Event{validSubmitted, {AggregateID: "repo#1", Version: 3, Type: EventStateChanged, From: StateInbox, To: StateTriage}}, want: ErrVersionGap},
		{name: "transition mismatch", events: []Event{validSubmitted, {AggregateID: "repo#1", Version: 2, Type: EventStateChanged, From: StateTriage, To: StateReady}}, want: ErrTransitionMismatch},
		{name: "unknown event type", events: []Event{validSubmitted, {AggregateID: "repo#1", Version: 2, Type: EventType("invented"), To: StateTriage}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Fold(test.events)
			if test.want == nil {
				if err == nil {
					t.Fatal("Fold() error = nil, want error")
				}
				return
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("Fold() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestFoldCapturesAndClearsBlockedResumeState(t *testing.T) {
	t.Parallel()

	events := []Event{
		{ID: "e1", AggregateID: "repo#1", Version: 1, Type: EventWorkSubmitted, To: StateInbox, At: time.Unix(1, 0)},
		{ID: "e2", AggregateID: "repo#1", Version: 2, Type: EventStateChanged, From: StateInbox, To: StateTriage, At: time.Unix(2, 0)},
		{ID: "e3", AggregateID: "repo#1", Version: 3, Type: EventStateChanged, From: StateTriage, To: StateBlocked, At: time.Unix(3, 0)},
	}

	blocked, err := Fold(events)
	if err != nil {
		t.Fatal(err)
	}
	if blocked.State != StateBlocked || blocked.ResumeState != StateTriage {
		t.Fatalf("blocked work item = %#v", blocked)
	}

	events = append(events, Event{ID: "e4", AggregateID: "repo#1", Version: 4, Type: EventStateChanged, From: StateBlocked, To: StateTriage, At: time.Unix(4, 0)})
	resumed, err := Fold(events)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.State != StateTriage || resumed.ResumeState != "" {
		t.Fatalf("resumed work item = %#v", resumed)
	}
}

func TestFoldRejectsResumeToDifferentState(t *testing.T) {
	t.Parallel()

	_, err := Fold([]Event{
		{ID: "e1", AggregateID: "repo#1", Version: 1, Type: EventWorkSubmitted, To: StateInbox, At: time.Unix(1, 0)},
		{ID: "e2", AggregateID: "repo#1", Version: 2, Type: EventStateChanged, From: StateInbox, To: StateTriage, At: time.Unix(2, 0)},
		{ID: "e3", AggregateID: "repo#1", Version: 3, Type: EventStateChanged, From: StateTriage, To: StateBlocked, At: time.Unix(3, 0)},
		{ID: "e4", AggregateID: "repo#1", Version: 4, Type: EventStateChanged, From: StateBlocked, To: StateReady, At: time.Unix(4, 0)},
	})
	if !errors.Is(err, ErrTransitionMismatch) {
		t.Fatalf("Fold() error = %v, want %v", err, ErrTransitionMismatch)
	}
}
