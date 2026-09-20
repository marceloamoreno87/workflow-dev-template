package workflow

import "testing"

func TestStateValid(t *testing.T) {
	t.Parallel()

	for _, state := range []State{
		StateInbox, StateTriage, StateReady, StateSpecifying,
		StateImplementing, StateReviewing, StateChangesRequested,
		StateReadyToDeploy, StateDeploying, StateClientQA,
		StateRollingOut, StateDone, StateBlocked, StateFailed,
		StateCancelled,
	} {
		if !state.Valid() {
			t.Fatalf("expected %q to be valid", state)
		}
	}

	if State("invented").Valid() {
		t.Fatal("unexpected valid invented state")
	}
}

func TestStringValueTypesValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{name: "work item ID", got: WorkItemID("work-item-1").Valid(), want: true},
		{name: "empty work item ID", got: WorkItemID("").Valid(), want: false},
		{name: "command ID", got: CommandID("command-1").Valid(), want: true},
		{name: "empty command ID", got: CommandID("").Valid(), want: false},
		{name: "actor ID", got: ActorID("actor-1").Valid(), want: true},
		{name: "empty actor ID", got: ActorID("").Valid(), want: false},
		{name: "command type", got: CommandType("start").Valid(), want: true},
		{name: "empty command type", got: CommandType("").Valid(), want: false},
		{name: "event type", got: EventType("started").Valid(), want: true},
		{name: "empty event type", got: EventType("").Valid(), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("Valid() = %t, want %t", test.got, test.want)
			}
		})
	}
}

func TestVersionValid(t *testing.T) {
	t.Parallel()

	if !Version(1).Valid() {
		t.Fatal("expected non-zero version to be valid")
	}

	if Version(0).Valid() {
		t.Fatal("unexpected valid zero version")
	}
}
