package flags

import (
	"testing"
	"time"
)

func TestExposureFlow(t *testing.T) {
	t.Parallel()

	flag := Flag{
		Key:     "checkout.redesign",
		Enabled: true,
		Rules: []TargetRule{
			{Name: "team", Attribute: "email", Values: []string{"@example.com"}, Percentage: 100, Variation: true},
		},
		Percentage: 1,
	}
	if err := flag.Validate(); err != nil {
		t.Fatal(err)
	}
	team, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "dev@example.com"}, BucketKey: "user-1"})
	if err != nil || !team {
		t.Fatalf("team should see the feature: %v %v", team, err)
	}
	stages := []RolloutStage{{Percentage: 1}, {Percentage: 10}, {Percentage: 50}, {Percentage: 100}}
	if err := ValidateRollout(stages); err != nil {
		t.Fatal(err)
	}
	advance, disable, err := DecideAcceptance(Acceptance{WorkItem: "owner/repo#123", Decision: "accept", By: "actor/client-a", ByKind: "contributor"})
	if err != nil || !advance || disable {
		t.Fatalf("acceptance should advance: %v %v %v", advance, disable, err)
	}
	completedAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if DebtOverdue(completedAt, completedAt.Add(13*24*time.Hour)) {
		t.Fatal("flag removed in time is not debt")
	}
	if !DebtOverdue(completedAt, completedAt.Add(15*24*time.Hour)) {
		t.Fatal("flag surviving past removal is debt")
	}
}

func TestRejectedAcceptanceDisables(t *testing.T) {
	t.Parallel()

	flag := validFlag()
	_, disable, err := DecideAcceptance(Acceptance{WorkItem: "owner/repo#123", Decision: "reject", By: "actor/client-a", ByKind: "contributor"})
	if err != nil || !disable {
		t.Fatalf("reject should disable: %v %v", disable, err)
	}
	flag.Enabled = false
	on, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "dev@example.com"}, BucketKey: "user-1"})
	if err != nil || on {
		t.Fatalf("disabled flag must stay off: %v %v", on, err)
	}
}
