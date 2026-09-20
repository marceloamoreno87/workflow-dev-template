package gatekeeper

import (
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestContributorMayAcceptOnlyDuringClientQA(t *testing.T) {
	t.Parallel()

	p := Policy{}
	allowed := p.Decide(Context{State: workflow.StateClientQA, Actor: ActorContributor, Profile: ProfileStandard}, workflow.Command{Type: workflow.CommandAcceptFeature})
	if !allowed.Allowed {
		t.Fatalf("expected allowed, got %#v", allowed)
	}

	denied := p.Decide(Context{State: workflow.StateReviewing, Actor: ActorContributor, Profile: ProfileStandard}, workflow.Command{Type: workflow.CommandApprovePR})
	if denied.Allowed || denied.Code != CodeActorDenied {
		t.Fatalf("expected actor denial, got %#v", denied)
	}
}

func TestPolicyDecisions(t *testing.T) {
	t.Parallel()

	operator := ActorOperator
	automation := ActorAutomation
	contributor := ActorContributor

	for _, test := range []struct {
		name    string
		context Context
		command workflow.Command
		want    Decision
	}{
		{
			name:    "operator may approve a standard merge",
			context: Context{State: workflow.StateReviewing, Actor: operator, Profile: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandApprovePR},
			want:    Decision{Allowed: true, Code: CodeAllowed},
		},
		{
			name:    "automation standard merge requires an operator human gate",
			context: Context{State: workflow.StateReviewing, Actor: automation, Profile: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandApprovePR},
			want:    Decision{Code: CodeHumanGateRequired},
		},
		{
			name:    "automation critical merge requires an operator human gate",
			context: Context{State: workflow.StateReviewing, Actor: automation, Profile: ProfileCritical},
			command: workflow.Command{Type: workflow.CommandApprovePR},
			want:    Decision{Code: CodeHumanGateRequired},
		},
		{
			name:    "contributor may accept during client QA",
			context: Context{State: workflow.StateClientQA, Actor: contributor, Profile: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandAcceptFeature},
			want:    Decision{Allowed: true, Code: CodeAllowed},
		},
		{
			name:    "contributor may not approve a PR",
			context: Context{State: workflow.StateReviewing, Actor: contributor, Profile: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandApprovePR},
			want:    Decision{Code: CodeActorDenied},
		},
		{
			name:    "automation may submit work for review",
			context: Context{State: workflow.StateImplementing, Actor: automation, Profile: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandSubmitReview},
			want:    Decision{Allowed: true, Code: CodeAllowed},
		},
		{
			name:    "automation may not authorize work",
			context: Context{State: workflow.StateTriage, Actor: automation, Profile: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandAuthorizeWork},
			want:    Decision{Code: CodeActorDenied},
		},
		{
			name:    "automation may not approve a spec",
			context: Context{State: workflow.StateSpecifying, Actor: automation, Profile: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandApproveSpec},
			want:    Decision{Code: CodeActorDenied},
		},
		{
			name:    "automation may not approve a prototype PR",
			context: Context{State: workflow.StateReviewing, Actor: automation, Profile: ProfilePrototype},
			command: workflow.Command{Type: workflow.CommandApprovePR},
			want:    Decision{Code: CodeActorDenied},
		},
		{
			name:    "automation may not accept a feature",
			context: Context{State: workflow.StateClientQA, Actor: automation, Profile: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandAcceptFeature},
			want:    Decision{Code: CodeActorDenied},
		},
		{
			name:    "automation may not change policy",
			context: Context{State: workflow.StateImplementing, Actor: automation, Profile: ProfileStandard, PolicyChange: true},
			command: workflow.Command{Type: workflow.CommandSubmitReview},
			want:    Decision{Code: CodeHumanGateRequired},
		},
		{
			name:    "effective profile may not be lower than the project minimum",
			context: Context{State: workflow.StateImplementing, Actor: automation, Profile: ProfilePrototype, ProjectMinimum: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandSubmitReview},
			want:    Decision{Code: CodeProfileDenied},
		},
		{
			name:    "standard production deploy requires an operator human gate",
			context: Context{State: workflow.StateReadyToDeploy, Actor: automation, Profile: ProfileStandard, Production: true},
			command: workflow.Command{Type: workflow.CommandBeginDeploy},
			want:    Decision{Code: CodeHumanGateRequired},
		},
		{
			name:    "critical deploy requires an operator human gate",
			context: Context{State: workflow.StateReadyToDeploy, Actor: automation, Profile: ProfileCritical},
			command: workflow.Command{Type: workflow.CommandBeginDeploy},
			want:    Decision{Code: CodeHumanGateRequired},
		},
		{
			name:    "invalid workflow state is denied",
			context: Context{State: workflow.StateDone, Actor: operator, Profile: ProfileStandard},
			command: workflow.Command{Type: workflow.CommandBeginDeploy},
			want:    Decision{Code: CodeStateDenied},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := (Policy{}).Decide(test.context, test.command)
			if got != test.want {
				t.Fatalf("Decide(%#v, %#v) = %#v, want %#v", test.context, test.command, got, test.want)
			}
		})
	}
}

func TestContributorMaySubmitOrCancelOnlyOwnInboxWork(t *testing.T) {
	t.Parallel()

	p := Policy{}
	submitted := p.Decide(Context{Actor: ActorContributor, Profile: ProfileStandard}, workflow.Command{Type: workflow.CommandSubmitWork})
	if submitted != (Decision{Allowed: true, Code: CodeAllowed}) {
		t.Fatalf("expected contributor submission to be allowed, got %#v", submitted)
	}

	resubmitted := p.Decide(Context{State: workflow.StateInbox, Actor: ActorContributor, Profile: ProfileStandard}, workflow.Command{Type: workflow.CommandSubmitWork})
	if resubmitted != (Decision{Code: CodeStateDenied}) {
		t.Fatalf("expected existing work submission to be state denied, got %#v", resubmitted)
	}

	for _, test := range []struct {
		name    string
		context Context
		want    Decision
	}{
		{
			name:    "their untriaged inbox work",
			context: Context{State: workflow.StateInbox, Actor: ActorContributor, Profile: ProfileStandard, OwnsWorkItem: true},
			want:    Decision{Allowed: true, Code: CodeAllowed},
		},
		{
			name:    "another contributor inbox work",
			context: Context{State: workflow.StateInbox, Actor: ActorContributor, Profile: ProfileStandard},
			want:    Decision{Code: CodeActorDenied},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := p.Decide(test.context, workflow.Command{Type: workflow.CommandCancel})
			if got != test.want {
				t.Fatalf("Decide() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestHighRiskActionsRequireOperator(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		context Context
	}{
		{name: "destructive migration", context: Context{DestructiveMigration: true}},
		{name: "secret or permission change", context: Context{SecretOrPermissionChange: true}},
		{name: "policy change", context: Context{PolicyChange: true}},
		{name: "harness self change", context: Context{HarnessSelfChange: true}},
		{name: "gate bypass", context: Context{BypassGate: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			context := test.context
			context.State = workflow.StateImplementing
			context.Actor = ActorAutomation
			context.Profile = ProfileStandard

			got := (Policy{}).Decide(context, workflow.Command{Type: workflow.CommandSubmitReview})
			want := Decision{Code: CodeHumanGateRequired}
			if got != want {
				t.Fatalf("Decide() = %#v, want %#v", got, want)
			}
		})
	}
}
