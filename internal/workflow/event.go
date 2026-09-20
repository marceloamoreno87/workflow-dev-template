package workflow

import "time"

const (
	EventWorkSubmitted EventType = "work_submitted"
	EventStateChanged  EventType = "state_changed"
)

type Event struct {
	ID          string
	AggregateID WorkItemID
	Version     Version
	CommandID   CommandID
	ActorID     ActorID
	Type        EventType
	From        State
	To          State
	Reason      string
	At          time.Time
}
