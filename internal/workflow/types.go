package workflow

type WorkItemID string
type CommandID string
type ActorID string
type Version uint64

func (id WorkItemID) Valid() bool {
	return id != ""
}

func (id CommandID) Valid() bool {
	return id != ""
}

func (id ActorID) Valid() bool {
	return id != ""
}

func (v Version) Valid() bool {
	return v != 0
}

type State string

const (
	StateInbox            State = "inbox"
	StateTriage           State = "triage"
	StateReady            State = "ready"
	StateSpecifying       State = "specifying"
	StateImplementing     State = "implementing"
	StateReviewing        State = "reviewing"
	StateChangesRequested State = "changes_requested"
	StateReadyToDeploy    State = "ready_to_deploy"
	StateDeploying        State = "deploying"
	StateClientQA         State = "client_qa"
	StateRollingOut       State = "rolling_out"
	StateDone             State = "done"
	StateBlocked          State = "blocked"
	StateFailed           State = "failed"
	StateCancelled        State = "cancelled"
)

func (s State) Valid() bool {
	switch s {
	case StateInbox, StateTriage, StateReady, StateSpecifying,
		StateImplementing, StateReviewing, StateChangesRequested,
		StateReadyToDeploy, StateDeploying, StateClientQA,
		StateRollingOut, StateDone, StateBlocked, StateFailed,
		StateCancelled:
		return true
	default:
		return false
	}
}

type CommandType string
type EventType string

func (t CommandType) Valid() bool {
	return t != ""
}

func (t EventType) Valid() bool {
	return t != ""
}
