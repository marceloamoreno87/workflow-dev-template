package flags

import (
	"testing"
	"time"
)

func TestValidateRollout(t *testing.T) {
	t.Parallel()

	stages := []RolloutStage{{Percentage: 1}, {Percentage: 10}, {Percentage: 50}, {Percentage: 100}}
	if err := ValidateRollout(stages); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]RolloutStage{
		nil,
		{{Percentage: 100}, {Percentage: 50}},
		{{Percentage: 10}, {Percentage: 10}},
		{{Percentage: 50}},
		{{Percentage: 0}},
		{{Percentage: 101}},
	} {
		if err := ValidateRollout(bad); err == nil {
			t.Fatalf("expected rejection for %v, got none", bad)
		}
	}
}

func TestAcceptanceDecides(t *testing.T) {
	t.Parallel()

	advance, disable, err := DecideAcceptance(Acceptance{WorkItem: "owner/repo#123", Decision: "accept", By: "actor/client-a", ByKind: "contributor"})
	if err != nil || !advance || disable {
		t.Fatalf("accept should advance: %v %v %v", advance, disable, err)
	}
	advance, disable, err = DecideAcceptance(Acceptance{WorkItem: "owner/repo#123", Decision: "reject", By: "actor/client-a", ByKind: "contributor"})
	if err != nil || advance || !disable {
		t.Fatalf("reject should disable: %v %v %v", advance, disable, err)
	}
	for _, bad := range []Acceptance{
		{WorkItem: "", Decision: "accept", By: "actor/client-a", ByKind: "contributor"},
		{WorkItem: "owner/repo#123", Decision: "maybe", By: "actor/client-a", ByKind: "contributor"},
		{WorkItem: "owner/repo#123", Decision: "accept", By: "actor/operator", ByKind: "operator"},
		{WorkItem: "owner/repo#123", Decision: "accept", By: "actor/automation", ByKind: "automation"},
	} {
		if _, _, err := DecideAcceptance(bad); err == nil {
			t.Fatalf("expected rejection for %#v, got none", bad)
		}
	}
}

func TestDebtDeadline(t *testing.T) {
	t.Parallel()

	done := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if got := RemovalDeadline(done); !got.Equal(done.Add(14 * 24 * time.Hour)) {
		t.Fatalf("unexpected deadline: %v", got)
	}
	if DebtOverdue(done, done.Add(13*24*time.Hour)) {
		t.Fatal("not overdue before 14 days")
	}
	if !DebtOverdue(done, done.Add(15*24*time.Hour)) {
		t.Fatal("overdue after 14 days")
	}
}
