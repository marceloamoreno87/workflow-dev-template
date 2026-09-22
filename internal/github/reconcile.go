// internal/github/reconcile.go
package github

import (
	"errors"
	"fmt"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

var ErrIntakeJump = errors.New("project status jump has no single command")

var ErrIntakeTerminal = errors.New("terminal work item cannot advance by intake")

type ReconcileResult struct {
	Commands       []workflow.Command
	IntentRecorded bool
}

const intakeActor = workflow.ActorID("actor/automation")

var stepCommand = map[[2]workflow.State]workflow.CommandType{
	{workflow.StateInbox, workflow.StateTriage}:                  workflow.CommandBeginTriage,
	{workflow.StateInbox, workflow.StateBlocked}:                 workflow.CommandBlock,
	{workflow.StateInbox, workflow.StateCancelled}:               workflow.CommandCancel,
	{workflow.StateTriage, workflow.StateReady}:                  workflow.CommandAuthorizeWork,
	{workflow.StateTriage, workflow.StateCancelled}:              workflow.CommandRejectWork,
	{workflow.StateTriage, workflow.StateBlocked}:                workflow.CommandBlock,
	{workflow.StateReady, workflow.StateSpecifying}:              workflow.CommandBeginSpec,
	{workflow.StateReady, workflow.StateImplementing}:            workflow.CommandBeginImplementation,
	{workflow.StateReady, workflow.StateBlocked}:                 workflow.CommandBlock,
	{workflow.StateReady, workflow.StateCancelled}:               workflow.CommandCancel,
	{workflow.StateSpecifying, workflow.StateReady}:              workflow.CommandApproveSpec,
	{workflow.StateSpecifying, workflow.StateBlocked}:            workflow.CommandBlock,
	{workflow.StateSpecifying, workflow.StateCancelled}:          workflow.CommandCancel,
	{workflow.StateImplementing, workflow.StateReviewing}:        workflow.CommandSubmitReview,
	{workflow.StateImplementing, workflow.StateBlocked}:          workflow.CommandBlock,
	{workflow.StateImplementing, workflow.StateCancelled}:        workflow.CommandCancel,
	{workflow.StateReviewing, workflow.StateChangesRequested}:    workflow.CommandRequestChanges,
	{workflow.StateReviewing, workflow.StateReadyToDeploy}:       workflow.CommandApprovePR,
	{workflow.StateReviewing, workflow.StateBlocked}:             workflow.CommandBlock,
	{workflow.StateReviewing, workflow.StateCancelled}:           workflow.CommandCancel,
	{workflow.StateChangesRequested, workflow.StateImplementing}: workflow.CommandBeginImplementation,
	{workflow.StateChangesRequested, workflow.StateCancelled}:    workflow.CommandCancel,
	{workflow.StateReadyToDeploy, workflow.StateDeploying}:       workflow.CommandBeginDeploy,
	{workflow.StateReadyToDeploy, workflow.StateBlocked}:         workflow.CommandBlock,
	{workflow.StateReadyToDeploy, workflow.StateCancelled}:       workflow.CommandCancel,
	{workflow.StateDeploying, workflow.StateClientQA}:            workflow.CommandMarkDeploymentHealthy,
	{workflow.StateDeploying, workflow.StateFailed}:              workflow.CommandFailDeployment,
	{workflow.StateClientQA, workflow.StateRollingOut}:           workflow.CommandAcceptFeature,
	{workflow.StateClientQA, workflow.StateChangesRequested}:     workflow.CommandRequestChanges,
	{workflow.StateClientQA, workflow.StateBlocked}:              workflow.CommandBlock,
	{workflow.StateRollingOut, workflow.StateDone}:               workflow.CommandCompleteRollout,
	{workflow.StateRollingOut, workflow.StateBlocked}:            workflow.CommandBlock,
	{workflow.StateFailed, workflow.StateReadyToDeploy}:          workflow.CommandRetryDeployment,
	{workflow.StateFailed, workflow.StateCancelled}:              workflow.CommandCancel,
}

func Reconcile(current workflow.WorkItem, issue IntakeIssue, projectStatus workflow.State) (ReconcileResult, error) {
	if issue.State == IssueClosed {
		return reconcileClosed(current)
	}
	if current == (workflow.WorkItem{}) {
		return ReconcileResult{Commands: []workflow.Command{{
			ID:              intakeCommandID(issue, "", workflow.StateInbox),
			AggregateID:     issue.ID,
			ExpectedVersion: 0,
			ActorID:         intakeActor,
			Type:            workflow.CommandSubmitWork,
			Reason:          issue.Title,
		}}}, nil
	}
	if current.State == workflow.StateDone || current.State == workflow.StateCancelled {
		return ReconcileResult{}, fmt.Errorf("%w: %q", ErrIntakeTerminal, current.State)
	}
	if current.State == workflow.StateBlocked {
		return ReconcileResult{}, fmt.Errorf("%w: resolve blocked explicitly", ErrIntakeJump)
	}
	if projectStatus == "" || projectStatus == current.State {
		return ReconcileResult{}, nil
	}
	cmdType, ok := stepCommand[[2]workflow.State{current.State, projectStatus}]
	if !ok {
		return ReconcileResult{}, fmt.Errorf("%w: %q to %q", ErrIntakeJump, current.State, projectStatus)
	}
	reason := fmt.Sprintf("github intake: %s -> %s", current.State, projectStatus)
	if needsReason(cmdType) {
		reason = issue.Title
	}
	return ReconcileResult{Commands: []workflow.Command{{
		ID:              intakeCommandID(issue, current.State, projectStatus),
		AggregateID:     current.ID,
		ExpectedVersion: current.Version,
		ActorID:         intakeActor,
		Type:            cmdType,
		Reason:          reason,
	}}}, nil
}

func reconcileClosed(current workflow.WorkItem) (ReconcileResult, error) {
	if current == (workflow.WorkItem{}) {
		return ReconcileResult{}, nil
	}
	if current.State == workflow.StateDone || current.State == workflow.StateCancelled {
		return ReconcileResult{}, nil
	}
	return ReconcileResult{IntentRecorded: true}, nil
}

func needsReason(cmdType workflow.CommandType) bool {
	switch cmdType {
	case workflow.CommandBlock, workflow.CommandCancel, workflow.CommandRejectWork, workflow.CommandFailDeployment:
		return true
	default:
		return false
	}
}

func intakeCommandID(issue IntakeIssue, from, to workflow.State) workflow.CommandID {
	if from == "" {
		return workflow.CommandID(fmt.Sprintf("intake-%d-submit", issue.Number))
	}
	return workflow.CommandID(fmt.Sprintf("intake-%d-%s-%s", issue.Number, from, to))
}
