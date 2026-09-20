package workflow

import (
	"errors"
	"fmt"
)

var (
	ErrEmptyHistory       = errors.New("empty event history")
	ErrAggregateMismatch  = errors.New("event aggregate mismatch")
	ErrVersionGap         = errors.New("event version gap")
	ErrTransitionMismatch = errors.New("event transition mismatch")
)

type WorkItem struct {
	ID          WorkItemID
	State       State
	ResumeState State
	Version     Version
}

func Fold(events []Event) (WorkItem, error) {
	if len(events) == 0 {
		return WorkItem{}, ErrEmptyHistory
	}

	first := events[0]
	if !first.AggregateID.Valid() {
		return WorkItem{}, ErrAggregateMismatch
	}
	if first.Version != 1 {
		return WorkItem{}, ErrVersionGap
	}
	if first.Type != EventWorkSubmitted || !first.To.Valid() {
		return WorkItem{}, ErrTransitionMismatch
	}

	workItem := WorkItem{
		ID:      first.AggregateID,
		State:   first.To,
		Version: first.Version,
	}

	for _, event := range events[1:] {
		if event.AggregateID != workItem.ID {
			return WorkItem{}, ErrAggregateMismatch
		}
		if event.Version != workItem.Version+1 {
			return WorkItem{}, ErrVersionGap
		}
		if event.Type != EventStateChanged {
			return WorkItem{}, fmt.Errorf("unknown event type: %q", event.Type)
		}
		if event.From != workItem.State || !event.To.Valid() {
			return WorkItem{}, ErrTransitionMismatch
		}

		if workItem.State != StateBlocked && event.To == StateBlocked {
			workItem.ResumeState = workItem.State
		} else if workItem.State == StateBlocked && event.To != StateBlocked {
			workItem.ResumeState = ""
		}

		workItem.State = event.To
		workItem.Version = event.Version
	}

	return workItem, nil
}
