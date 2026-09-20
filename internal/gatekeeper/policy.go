package gatekeeper

import "github.com/marceloamoreno87/workflow-dev-template/internal/workflow"

type Profile string

const (
	ProfilePrototype Profile = "prototype"
	ProfileStandard  Profile = "standard"
	ProfileCritical  Profile = "critical"
)

type ActorKind string

const (
	ActorOperator    ActorKind = "operator"
	ActorContributor ActorKind = "contributor"
	ActorAutomation  ActorKind = "automation"
)

type DecisionCode string

const (
	CodeAllowed           DecisionCode = "allowed"
	CodeActorDenied       DecisionCode = "actor_denied"
	CodeHumanGateRequired DecisionCode = "human_gate_required"
	CodeProfileDenied     DecisionCode = "profile_denied"
	CodeStateDenied       DecisionCode = "state_denied"
)

type Decision struct {
	Allowed bool
	Code    DecisionCode
}

type Context struct {
	State                    workflow.State
	Actor                    ActorKind
	Profile                  Profile
	ProjectMinimum           Profile
	Production               bool
	DestructiveMigration     bool
	SecretOrPermissionChange bool
	PolicyChange             bool
	HarnessSelfChange        bool
	BypassGate               bool
	OwnsWorkItem             bool
}

type Policy struct{}

func (Policy) Decide(context Context, command workflow.Command) Decision {
	if !allowedInState(context.State, command.Type) {
		return Decision{Code: CodeStateDenied}
	}
	if profileLowerThan(context.Profile, context.ProjectMinimum) {
		return Decision{Code: CodeProfileDenied}
	}
	switch context.Actor {
	case ActorOperator:
		return Decision{Allowed: true, Code: CodeAllowed}
	case ActorContributor:
		if !contributorAllowed(context, command.Type) {
			return Decision{Code: CodeActorDenied}
		}
		if requiresOperatorGate(context, command.Type) {
			return Decision{Code: CodeHumanGateRequired}
		}
		return Decision{Allowed: true, Code: CodeAllowed}
	case ActorAutomation:
		if requiresOperatorGate(context, command.Type) {
			return Decision{Code: CodeHumanGateRequired}
		}
		if !automationAllowed(command.Type) {
			return Decision{Code: CodeActorDenied}
		}
		return Decision{Allowed: true, Code: CodeAllowed}
	}
	return Decision{Code: CodeActorDenied}
}

func profileLowerThan(profile, minimum Profile) bool {
	return profileRank(profile) < profileRank(minimum)
}

func profileRank(profile Profile) int {
	switch profile {
	case ProfilePrototype:
		return 1
	case ProfileStandard:
		return 2
	case ProfileCritical:
		return 3
	default:
		return 0
	}
}

func requiresOperatorGate(context Context, commandType workflow.CommandType) bool {
	if context.DestructiveMigration || context.SecretOrPermissionChange || context.PolicyChange || context.HarnessSelfChange || context.BypassGate {
		return true
	}
	if commandType == workflow.CommandApprovePR && context.Profile != ProfilePrototype {
		return true
	}
	if commandType != workflow.CommandBeginDeploy {
		return false
	}
	return context.Profile == ProfileCritical || (context.Profile == ProfileStandard && context.Production)
}

func contributorAllowed(context Context, commandType workflow.CommandType) bool {
	switch commandType {
	case workflow.CommandSubmitWork:
		return context.State == ""
	case workflow.CommandAcceptFeature, workflow.CommandRequestChanges:
		return context.State == workflow.StateClientQA
	case workflow.CommandCancel:
		return context.State == workflow.StateInbox && context.OwnsWorkItem
	default:
		return false
	}
}

func automationAllowed(commandType workflow.CommandType) bool {
	switch commandType {
	case workflow.CommandAuthorizeWork, workflow.CommandApproveSpec, workflow.CommandApprovePR, workflow.CommandAcceptFeature:
		return false
	default:
		return true
	}
}

func allowedInState(state workflow.State, commandType workflow.CommandType) bool {
	for _, allowed := range (workflow.Workflow{}).Allowed(workflow.WorkItem{State: state}) {
		if allowed == commandType {
			return true
		}
	}
	return false
}
