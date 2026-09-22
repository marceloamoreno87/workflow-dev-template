// internal/delivery/evidence.go
package delivery

import (
	"fmt"
	"time"
)

type Evidence struct {
	CommandID     string
	Tool          string
	Version       string
	StartedAt     time.Time
	FinishedAt    time.Time
	ExitCode      int
	Commit        string
	Applicable    bool
	Deterministic bool
	Note          string
}

func (e Evidence) Validate() error {
	if e.CommandID == "" {
		return fmt.Errorf("%w: command identity required", ErrDelivery)
	}
	if e.Tool == "" || e.Version == "" {
		return fmt.Errorf("%w: tool identity and version required", ErrDelivery)
	}
	if e.StartedAt.IsZero() || e.FinishedAt.IsZero() {
		return fmt.Errorf("%w: evidence window required", ErrDelivery)
	}
	if e.FinishedAt.Before(e.StartedAt) {
		return fmt.Errorf("%w: evidence window inverted", ErrDelivery)
	}
	if e.ExitCode < 0 {
		return fmt.Errorf("%w: negative exit code", ErrDelivery)
	}
	if !e.Applicable && e.Note == "" {
		return fmt.Errorf("%w: not-applicable evidence needs a reason", ErrDelivery)
	}
	return nil
}
