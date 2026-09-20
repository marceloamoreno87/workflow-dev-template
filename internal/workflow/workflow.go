package workflow

import (
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrCommandIDRequired        = errors.New("command ID required")
	ErrCommandAggregateRequired = errors.New("command aggregate ID required")
	ErrCommandActorRequired     = errors.New("command actor ID required")
	ErrCommandAggregateMismatch = errors.New("command aggregate mismatch")
	ErrCommandVersionStale      = errors.New("command version stale")
	ErrCommandReasonRequired    = errors.New("command reason required")
	ErrTerminalState            = errors.New("terminal state")
	ErrTransitionNotAllowed     = errors.New("transition not allowed")
)

var transitions = map[State]map[CommandType]State{
	StateInbox:            {CommandBeginTriage: StateTriage, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateTriage:           {CommandAuthorizeWork: StateReady, CommandRejectWork: StateCancelled, CommandBlock: StateBlocked},
	StateReady:            {CommandBeginSpec: StateSpecifying, CommandBeginImplementation: StateImplementing, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateSpecifying:       {CommandApproveSpec: StateReady, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateImplementing:     {CommandSubmitReview: StateReviewing, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateReviewing:        {CommandRequestChanges: StateChangesRequested, CommandApprovePR: StateReadyToDeploy, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateChangesRequested: {CommandBeginImplementation: StateImplementing, CommandCancel: StateCancelled},
	StateReadyToDeploy:    {CommandBeginDeploy: StateDeploying, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateDeploying:        {CommandMarkDeploymentHealthy: StateClientQA, CommandFailDeployment: StateFailed},
	StateClientQA:         {CommandAcceptFeature: StateRollingOut, CommandRequestChanges: StateChangesRequested, CommandBlock: StateBlocked},
	StateRollingOut:       {CommandCompleteRollout: StateDone, CommandBlock: StateBlocked},
	StateFailed:           {CommandRetryDeployment: StateReadyToDeploy, CommandCancel: StateCancelled},
}

type Workflow struct{}

func (Workflow) Handle(now time.Time, item WorkItem, command Command) ([]Event, error) {
	if err := validateCommand(item, command); err != nil {
		return nil, err
	}

	if command.Type == CommandSubmitWork {
		if item != (WorkItem{}) {
			return nil, ErrTransitionNotAllowed
		}
		return []Event{{AggregateID: command.AggregateID, Version: 1, CommandID: command.ID, ActorID: command.ActorID, Type: EventWorkSubmitted, To: StateInbox, Reason: command.Reason, At: now}}, nil
	}

	if item.State == StateDone || item.State == StateCancelled {
		return nil, ErrTerminalState
	}

	to, allowed := transitions[item.State][command.Type]
	if command.Type == CommandResolveBlock && item.State == StateBlocked {
		to = item.ResumeState
		allowed = to.Valid() && to != StateBlocked
	}
	if !allowed {
		return nil, ErrTransitionNotAllowed
	}

	return []Event{{AggregateID: item.ID, Version: item.Version + 1, CommandID: command.ID, ActorID: command.ActorID, Type: EventStateChanged, From: item.State, To: to, Reason: command.Reason, At: now}}, nil
}

func (Workflow) Allowed(item WorkItem) []CommandType {
	if item == (WorkItem{}) {
		return []CommandType{CommandSubmitWork}
	}
	if item.State == StateBlocked {
		return []CommandType{CommandResolveBlock}
	}

	allowed := make([]CommandType, 0, len(transitions[item.State]))
	for command := range transitions[item.State] {
		allowed = append(allowed, command)
	}
	sort.Slice(allowed, func(i, j int) bool { return allowed[i] < allowed[j] })
	return allowed
}

func validateCommand(item WorkItem, command Command) error {
	if !command.ID.Valid() {
		return ErrCommandIDRequired
	}
	if !command.AggregateID.Valid() {
		return ErrCommandAggregateRequired
	}
	if !command.ActorID.Valid() {
		return ErrCommandActorRequired
	}
	if command.Type != CommandSubmitWork && command.AggregateID != item.ID {
		return ErrCommandAggregateMismatch
	}
	if command.ExpectedVersion != item.Version {
		return ErrCommandVersionStale
	}
	if requiresReason(command.Type) && strings.TrimSpace(command.Reason) == "" {
		return ErrCommandReasonRequired
	}
	return nil
}

func requiresReason(commandType CommandType) bool {
	switch commandType {
	case CommandBlock, CommandCancel, CommandRejectWork, CommandFailDeployment:
		return true
	default:
		return false
	}
}
