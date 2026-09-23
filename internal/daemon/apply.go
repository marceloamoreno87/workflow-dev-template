// internal/daemon/apply.go
package daemon

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/dashboard"
	"github.com/marceloamoreno87/workflow-dev-template/internal/gatekeeper"
	"github.com/marceloamoreno87/workflow-dev-template/internal/journal"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

// ApplyOperatorCommand validates a dashboard-originated command as the operator,
// commits it to the journal, and returns the new version. Stale versions fail
// with ErrApplyConflict carrying the currently allowed actions.
func (d *Daemon) ApplyOperatorCommand(req dashboard.CommandRequest) (uint64, error) {
	item, err := d.db.Load(workflow.WorkItemID(req.AggregateID))
	if err != nil && !errors.Is(err, journal.ErrNotFound) {
		return 0, err
	}
	cmd := workflow.Command{
		ID:              commandID(),
		AggregateID:     workflow.WorkItemID(req.AggregateID),
		ExpectedVersion: workflow.Version(req.ExpectedVersion),
		ActorID:         "actor/operator",
		Type:            req.Type,
		Reason:          req.Reason,
	}
	decision := (gatekeeper.Policy{}).Decide(gatekeeper.Context{
		State: item.State, Actor: gatekeeper.ActorOperator,
		Profile: gatekeeper.ProfileStandard, ProjectMinimum: gatekeeper.ProfilePrototype,
	}, cmd)
	if !decision.Allowed {
		return 0, fmt.Errorf("%w: %s", dashboard.ErrApplyRejected, decision.Code)
	}
	if _, err := d.db.Apply(time.Now(), cmd); err != nil {
		if errors.Is(err, journal.ErrConflict) {
			allowed := (workflow.Workflow{}).Allowed(item)
			names := make([]string, 0, len(allowed))
			for _, name := range allowed {
				names = append(names, string(name))
			}
			return 0, fmt.Errorf("%w: %s", dashboard.ErrApplyConflict, strings.Join(names, ","))
		}
		return 0, err
	}
	applied, err := d.db.Load(workflow.WorkItemID(req.AggregateID))
	if err != nil {
		return 0, err
	}
	d.feedDashboard()
	return uint64(applied.Version), nil
}
