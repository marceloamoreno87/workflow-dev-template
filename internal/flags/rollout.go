// internal/flags/rollout.go
package flags

import (
	"fmt"
	"time"
)

type RolloutStage struct {
	Percentage float64
}

func ValidateRollout(stages []RolloutStage) error {
	if len(stages) == 0 {
		return fmt.Errorf("%w: rollout needs stages", ErrFlag)
	}
	previous := -1.0
	for _, stage := range stages {
		if !(stage.Percentage > 0 && stage.Percentage <= 100) {
			return fmt.Errorf("%w: stage percentage %v", ErrFlag, stage.Percentage)
		}
		if stage.Percentage <= previous {
			return fmt.Errorf("%w: stages must strictly increase", ErrFlag)
		}
		previous = stage.Percentage
	}
	if previous != 100 {
		return fmt.Errorf("%w: rollout must end at 100", ErrFlag)
	}
	return nil
}

type Acceptance struct {
	WorkItem string
	Decision string
	By       string
	ByKind   string
}

func DecideAcceptance(a Acceptance) (advance, disable bool, err error) {
	if a.WorkItem == "" || a.By == "" {
		return false, false, fmt.Errorf("%w: work item and acceptor required", ErrFlag)
	}
	if a.ByKind != "contributor" {
		return false, false, fmt.Errorf("%w: acceptance belongs to contributors", ErrFlag)
	}
	switch a.Decision {
	case "accept":
		return true, false, nil
	case "reject":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("%w: decision %q", ErrFlag, a.Decision)
	}
}

func RemovalDeadline(completedAt time.Time) time.Time {
	return completedAt.Add(14 * 24 * time.Hour)
}

func DebtOverdue(completedAt, now time.Time) bool {
	return !now.Before(RemovalDeadline(completedAt))
}
