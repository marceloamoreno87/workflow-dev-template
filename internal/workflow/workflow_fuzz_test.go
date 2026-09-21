package workflow

import (
	"testing"
	"time"
)

var fuzzStates = []State{
	StateInbox, StateTriage, StateReady, StateSpecifying,
	StateImplementing, StateReviewing, StateChangesRequested,
	StateReadyToDeploy, StateDeploying, StateClientQA,
	StateRollingOut, StateDone, StateBlocked, StateFailed,
	StateCancelled, State("invalid"), State(""),
}

var fuzzEventTypes = []EventType{
	EventWorkSubmitted, EventStateChanged, EventType("invented"), EventType(""),
}

func FuzzFoldNeverReturnsInvalidState(f *testing.F) {
	f.Add([]byte{1, 0, 0, 1, 0, 1})
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8})
	f.Add([]byte{255, 255, 255, 255})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			if _, err := Fold(nil); err == nil {
				t.Fatal("Fold(nil) expected error")
			}
			return
		}

		eventCount := int(data[0]%8) + 1
		events := make([]Event, 0, eventCount)
		version := Version(0)

		for i := 0; i < eventCount; i++ {
			b0, b1, b2, b3 := byte(i), byte(i), byte(i), byte(i)
			if i < len(data) {
				b0 = data[i]
			}
			if (i + 1) < len(data) {
				b1 = data[i+1]
			}
			if (i + 2) < len(data) {
				b2 = data[i+2]
			}
			if (i + 3) < len(data) {
				b3 = data[i+3]
			}

			if i == 0 {
				// First event: mostly submission, sometimes malformed.
				version = Version(b0 % 3)
				events = append(events, Event{
					ID:          "fuzz-e0",
					AggregateID: WorkItemID("fuzz#1"),
					Version:     version,
					Type:        fuzzEventTypes[int(b1)%len(fuzzEventTypes)],
					To:          fuzzStates[int(b2)%len(fuzzStates)],
					At:          time.Unix(1, 0),
				})
				continue
			}

			// Later events: version may stay, +1, or +2 to exercise gaps.
			version += Version(b0 % 3)
			from := fuzzStates[int(b1)%len(fuzzStates)]
			to := fuzzStates[int(b2)%len(fuzzStates)]
			eventType := EventStateChanged
			if b3%4 == 3 {
				eventType = fuzzEventTypes[int(b3)%len(fuzzEventTypes)]
			}
			aggregateID := WorkItemID("fuzz#1")
			if b3%7 == 6 {
				aggregateID = WorkItemID("fuzz#2")
			}
			events = append(events, Event{
				ID:          string(rune('a' + i)),
				AggregateID: aggregateID,
				Version:     version,
				Type:        eventType,
				From:        from,
				To:          to,
				At:          time.Unix(int64(i+1), 0),
			})
		}

		got, err := Fold(events)
		if err != nil {
			return
		}
		if got.Version != events[len(events)-1].Version {
			t.Fatalf("Fold() Version = %d, want final event version %d", got.Version, events[len(events)-1].Version)
		}
		if !got.State.Valid() {
			t.Fatalf("Fold() State = %q, want valid state", got.State)
		}
	})
}
