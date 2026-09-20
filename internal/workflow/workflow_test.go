package workflow

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestHandleBeginTriage(t *testing.T) {
	t.Parallel()

	w := Workflow{}
	item := WorkItem{ID: "repo#1", State: StateInbox, Version: 1}
	command := Command{ID: "c2", AggregateID: item.ID, ExpectedVersion: 1, ActorID: "operator", Type: CommandBeginTriage}

	events, err := w.Handle(time.Unix(2, 0), item, command)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].From != StateInbox || events[0].To != StateTriage || events[0].Version != 2 {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestHandleTransitions(t *testing.T) {
	t.Parallel()

	w := Workflow{}
	now := time.Unix(2, 0)
	for from, commands := range transitions {
		for commandType, to := range commands {
			t.Run(string(from)+"/"+string(commandType), func(t *testing.T) {
				reason := ""
				if requiresReason(commandType) {
					reason = "required"
				}
				item := WorkItem{ID: "repo#1", State: from, Version: 1}
				command := Command{ID: "c2", AggregateID: item.ID, ExpectedVersion: item.Version, ActorID: "operator", Type: commandType, Reason: reason}

				events, err := w.Handle(now, item, command)
				if err != nil {
					t.Fatal(err)
				}
				want := Event{AggregateID: item.ID, Version: 2, CommandID: command.ID, ActorID: command.ActorID, Type: EventStateChanged, From: from, To: to, Reason: reason, At: now}
				if !reflect.DeepEqual(events, []Event{want}) {
					t.Fatalf("Handle() events = %#v, want %#v", events, []Event{want})
				}
			})
		}
	}
}

func TestHandleRejectsInvalidTransitionSources(t *testing.T) {
	t.Parallel()

	states := []State{StateInbox, StateTriage, StateReady, StateSpecifying, StateImplementing, StateReviewing, StateChangesRequested, StateReadyToDeploy, StateDeploying, StateClientQA, StateRollingOut, StateBlocked, StateFailed}
	for _, from := range states {
		for commandType := range allCommandTypes() {
			if _, allowed := transitions[from][commandType]; allowed {
				continue
			}
			if from == StateBlocked && commandType == CommandResolveBlock {
				continue
			}
			t.Run(string(from)+"/"+string(commandType), func(t *testing.T) {
				item := WorkItem{ID: "repo#1", State: from, Version: 1}
				command := Command{ID: "c2", AggregateID: item.ID, ExpectedVersion: item.Version, ActorID: "operator", Type: commandType, Reason: "required"}

				events, err := (Workflow{}).Handle(time.Unix(2, 0), item, command)
				if !errors.Is(err, ErrTransitionNotAllowed) {
					t.Fatalf("Handle() error = %v, want %v", err, ErrTransitionNotAllowed)
				}
				if len(events) != 0 {
					t.Fatalf("Handle() events = %#v, want none", events)
				}
			})
		}
	}
}

func TestHandleSubmitsOnlyNewWork(t *testing.T) {
	t.Parallel()

	w := Workflow{}
	command := Command{ID: "c1", AggregateID: "repo#1", ActorID: "operator", Type: CommandSubmitWork}
	events, err := w.Handle(time.Unix(1, 0), WorkItem{}, command)
	if err != nil {
		t.Fatal(err)
	}
	want := []Event{{AggregateID: "repo#1", Version: 1, CommandID: "c1", ActorID: "operator", Type: EventWorkSubmitted, To: StateInbox, At: time.Unix(1, 0)}}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("Handle() events = %#v, want %#v", events, want)
	}

	events, err = w.Handle(time.Unix(1, 0), WorkItem{ID: "repo#1", State: StateInbox, Version: 1}, Command{ID: "c1", AggregateID: "repo#1", ExpectedVersion: 1, ActorID: "operator", Type: CommandSubmitWork})
	if !errors.Is(err, ErrTransitionNotAllowed) || len(events) != 0 {
		t.Fatalf("Handle() = %#v, %v; want no events and %v", events, err, ErrTransitionNotAllowed)
	}
}

func TestHandleResolvesBlockToCapturedState(t *testing.T) {
	t.Parallel()

	item := WorkItem{ID: "repo#1", State: StateBlocked, ResumeState: StateReviewing, Version: 3}
	command := Command{ID: "c4", AggregateID: item.ID, ExpectedVersion: item.Version, ActorID: "operator", Type: CommandResolveBlock}
	events, err := (Workflow{}).Handle(time.Unix(4, 0), item, command)
	if err != nil {
		t.Fatal(err)
	}
	want := []Event{{AggregateID: item.ID, Version: 4, CommandID: command.ID, ActorID: command.ActorID, Type: EventStateChanged, From: StateBlocked, To: StateReviewing, At: time.Unix(4, 0)}}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("Handle() events = %#v, want %#v", events, want)
	}
}

func TestHandleRejectsMalformedCommands(t *testing.T) {
	t.Parallel()

	item := WorkItem{ID: "repo#1", State: StateInbox, Version: 1}
	valid := Command{ID: "c2", AggregateID: item.ID, ExpectedVersion: item.Version, ActorID: "operator", Type: CommandBeginTriage}
	tests := []struct {
		name    string
		item    WorkItem
		command Command
		want    error
	}{
		{name: "empty command ID", item: item, command: Command{AggregateID: item.ID, ExpectedVersion: item.Version, ActorID: "operator", Type: CommandBeginTriage}, want: ErrCommandIDRequired},
		{name: "empty aggregate ID", item: item, command: Command{ID: valid.ID, ExpectedVersion: item.Version, ActorID: "operator", Type: CommandBeginTriage}, want: ErrCommandAggregateRequired},
		{name: "empty actor ID", item: item, command: Command{ID: valid.ID, AggregateID: item.ID, ExpectedVersion: item.Version, Type: CommandBeginTriage}, want: ErrCommandActorRequired},
		{name: "aggregate mismatch", item: item, command: Command{ID: valid.ID, AggregateID: "repo#2", ExpectedVersion: item.Version, ActorID: valid.ActorID, Type: valid.Type}, want: ErrCommandAggregateMismatch},
		{name: "stale version", item: item, command: Command{ID: valid.ID, AggregateID: item.ID, ExpectedVersion: 2, ActorID: valid.ActorID, Type: valid.Type}, want: ErrCommandVersionStale},
		{name: "block without reason", item: item, command: Command{ID: valid.ID, AggregateID: item.ID, ExpectedVersion: item.Version, ActorID: valid.ActorID, Type: CommandBlock}, want: ErrCommandReasonRequired},
		{name: "cancel with blank reason", item: item, command: Command{ID: valid.ID, AggregateID: item.ID, ExpectedVersion: item.Version, ActorID: valid.ActorID, Type: CommandCancel, Reason: " \t"}, want: ErrCommandReasonRequired},
		{name: "reject without reason", item: WorkItem{ID: item.ID, State: StateTriage, Version: 1}, command: Command{ID: valid.ID, AggregateID: item.ID, ExpectedVersion: 1, ActorID: valid.ActorID, Type: CommandRejectWork}, want: ErrCommandReasonRequired},
		{name: "fail without reason", item: WorkItem{ID: item.ID, State: StateDeploying, Version: 1}, command: Command{ID: valid.ID, AggregateID: item.ID, ExpectedVersion: 1, ActorID: valid.ActorID, Type: CommandFailDeployment}, want: ErrCommandReasonRequired},
		{name: "done terminal", item: WorkItem{ID: item.ID, State: StateDone, Version: 1}, command: Command{ID: valid.ID, AggregateID: item.ID, ExpectedVersion: 1, ActorID: valid.ActorID, Type: CommandBeginTriage}, want: ErrTerminalState},
		{name: "cancelled terminal", item: WorkItem{ID: item.ID, State: StateCancelled, Version: 1}, command: Command{ID: valid.ID, AggregateID: item.ID, ExpectedVersion: 1, ActorID: valid.ActorID, Type: CommandBeginTriage}, want: ErrTerminalState},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events, err := (Workflow{}).Handle(time.Unix(2, 0), test.item, test.command)
			if !errors.Is(err, test.want) {
				t.Fatalf("Handle() error = %v, want %v", err, test.want)
			}
			if len(events) != 0 {
				t.Fatalf("Handle() events = %#v, want none", events)
			}
		})
	}
}

func TestAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state State
		want  []CommandType
	}{
		{state: StateInbox, want: []CommandType{CommandBeginTriage, CommandBlock, CommandCancel}},
		{state: StateReviewing, want: []CommandType{CommandApprovePR, CommandBlock, CommandCancel, CommandRequestChanges}},
		{state: StateClientQA, want: []CommandType{CommandAcceptFeature, CommandBlock, CommandRequestChanges}},
		{state: StateBlocked, want: []CommandType{CommandResolveBlock}},
		{state: StateDone, want: []CommandType{}},
		{state: StateCancelled, want: []CommandType{}},
	}

	for _, test := range tests {
		t.Run(string(test.state), func(t *testing.T) {
			got := (Workflow{}).Allowed(WorkItem{State: test.state})
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Allowed() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func allCommandTypes() map[CommandType]struct{} {
	commands := make(map[CommandType]struct{})
	for _, stateCommands := range transitions {
		for command := range stateCommands {
			commands[command] = struct{}{}
		}
	}
	commands[CommandResolveBlock] = struct{}{}
	return commands
}
