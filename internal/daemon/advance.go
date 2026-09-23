// internal/daemon/advance.go
package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/gatekeeper"
	"github.com/marceloamoreno87/workflow-dev-template/internal/journal"
	"github.com/marceloamoreno87/workflow-dev-template/internal/loop"
	"github.com/marceloamoreno87/workflow-dev-template/internal/registry"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workspace"
)

func commandID() workflow.CommandID {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return workflow.CommandID(fmt.Sprintf("daemon-%d", time.Now().UnixNano()))
	}
	return workflow.CommandID("daemon-" + hex.EncodeToString(raw[:]))
}

func (d *Daemon) applyWorkflow(id workflow.WorkItemID, cmdType workflow.CommandType, reason string, now time.Time) error {
	item, err := d.db.Load(id)
	if err != nil {
		return err
	}
	cmd := workflow.Command{
		ID:              commandID(),
		AggregateID:     id,
		ExpectedVersion: item.Version,
		ActorID:         "actor/automation",
		Type:            cmdType,
		Reason:          reason,
	}
	decision := (gatekeeper.Policy{}).Decide(gatekeeper.Context{
		State: item.State, Actor: gatekeeper.ActorAutomation,
		Profile: gatekeeper.ProfileStandard, ProjectMinimum: gatekeeper.ProfilePrototype,
	}, cmd)
	if !decision.Allowed {
		return fmt.Errorf("%w: %s denied (%s)", ErrDaemon, cmdType, decision.Code)
	}
	_, err = d.db.Apply(now, cmd)
	return err
}

// resolveProject maps a Work Item to its registered Project. found=false means the
// item waits: unregistered repositories skip the tick without error.
func (d *Daemon) resolveProject(id workflow.WorkItemID) (projectID, submodulePath string, manifest registry.ProjectManifest, found bool, err error) {
	repo, _, ok := strings.Cut(string(id), "#")
	if !ok || repo == "" {
		return "", "", registry.ProjectManifest{}, false, fmt.Errorf("%w: malformed work item id %q", ErrDaemon, id)
	}
	raw, err := os.ReadFile(filepath.Join(harnessDir(d.cfg.WorkspaceRoot), "registry.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", registry.ProjectManifest{}, false, nil
		}
		return "", "", registry.ProjectManifest{}, false, err
	}
	reg, err := registry.ParseRegistry(raw)
	if err != nil {
		return "", "", registry.ProjectManifest{}, false, err
	}
	if err := reg.Validate(); err != nil {
		return "", "", registry.ProjectManifest{}, false, err
	}
	var relPath string
	for _, p := range reg.Projects {
		if p.Github.Repository == repo {
			projectID = p.ID
			relPath = filepath.FromSlash(p.Path)
			break
		}
	}
	if projectID == "" {
		return "", "", registry.ProjectManifest{}, false, nil
	}
	submodulePath = filepath.Join(d.cfg.WorkspaceRoot, relPath)
	manifestRaw, err := os.ReadFile(filepath.Join(submodulePath, ".harness", "project.yaml"))
	if err != nil {
		return "", "", registry.ProjectManifest{}, false, err
	}
	if err := registry.RejectSecrets(manifestRaw); err != nil {
		return "", "", registry.ProjectManifest{}, false, err
	}
	manifest, err = registry.ParseProjectManifest(manifestRaw)
	if err != nil {
		return "", "", registry.ProjectManifest{}, false, err
	}
	if err := manifest.Validate(); err != nil {
		return "", "", registry.ProjectManifest{}, false, err
	}
	return projectID, submodulePath, manifest, true, nil
}

func (d *Daemon) advanceItem(ctx context.Context, id workflow.WorkItemID, now time.Time) (bool, error) {
	item, err := d.db.Load(id)
	if err != nil {
		if errors.Is(err, journal.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	switch item.State {
	case workflow.StateInbox:
		if err := d.applyWorkflow(id, workflow.CommandBeginTriage, "", now); err != nil {
			return false, err
		}
		return true, nil
	case workflow.StateTriage:
		return false, nil
	case workflow.StateReady, workflow.StateImplementing, workflow.StateReviewing:
		projectID, sub, manifest, found, err := d.resolveProject(id)
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}
		rec, ok := d.loops[id]
		if !ok {
			rec = LoopRecord{ProjectID: projectID}
		}
		if rec.Loop.Stage == "" {
			begun, err := loop.Begin(string(id), d.cfg.RequiredRoles, d.cfg.BudgetUSD, d.cfg.MaxDuration, now)
			if err != nil {
				return false, err
			}
			rec.Loop = begun
			rec.ProjectID = projectID
			if err := d.saveLoop(id, rec); err != nil {
				return false, err
			}
		}
		worked, err := d.advanceLoop(ctx, id, rec, projectID, sub, manifest, now)
		if err != nil {
			return worked, err
		}
		return worked, nil
	default:
		return false, nil
	}
}

// closeIfPresent removes the worktree when its directory still exists, making
// terminal handling idempotent across restarts.
func (d *Daemon) closeIfPresent(projectID, submodulePath string) (bool, error) {
	dir := filepath.Join(d.cfg.WorkspaceRoot, ".worktrees", projectID)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := workspace.Close(d.cfg.WorkspaceRoot, projectID, submodulePath); err != nil {
		return false, err
	}
	return true, nil
}

func (d *Daemon) mirrorBlock(id workflow.WorkItemID, item workflow.WorkItem, reason string, now time.Time) (bool, error) {
	switch item.State {
	case workflow.StateBlocked, workflow.StateDone, workflow.StateCancelled:
		return false, nil
	}
	if strings.TrimSpace(reason) == "" {
		return false, fmt.Errorf("%w: block without reason", ErrDaemon)
	}
	if err := d.applyWorkflow(id, workflow.CommandBlock, reason, now); err != nil {
		return false, err
	}
	return true, nil
}

func (d *Daemon) feed(id workflow.WorkItemID, rec *LoopRecord, event loop.LoopEvent, now time.Time) (loop.Decision, error) {
	next, decision, err := loop.Advance(rec.Loop, event, now)
	if err != nil {
		return loop.Decision{}, err
	}
	rec.Loop = next
	if err := d.saveLoop(id, *rec); err != nil {
		return loop.Decision{}, err
	}
	return decision, nil
}

// advanceLoop moves one (workflow, loop) pair forward. Workflow commands land before
// the corresponding loop feed, so a crash between them always resumes consistently.
func (d *Daemon) advanceLoop(ctx context.Context, id workflow.WorkItemID, rec LoopRecord, projectID, submodulePath string, manifest registry.ProjectManifest, now time.Time) (bool, error) {
	item, err := d.db.Load(id)
	if err != nil {
		return false, err
	}
	work := false

	if rec.Loop.Stage == loop.StageDone {
		closed, err := d.closeIfPresent(projectID, submodulePath)
		return closed, err
	}
	if rec.Loop.Stage == loop.StageBlocked {
		return d.mirrorBlock(id, item, rec.Loop.BlockedReason, now)
	}

	if rec.Loop.Stage == loop.StageSpecifying {
		switch item.State {
		case workflow.StateReady:
			if !rec.ProductDone {
				outcome, err := d.ExecuteRole(ctx, id, rec, projectID, submodulePath, manifest, "product", now)
				if err != nil {
					return work, err
				}
				_ = outcome
				rec.ProductDone = true
				if err := d.saveLoop(id, rec); err != nil {
					return work, err
				}
				work = true
			}
			if _, err := d.feed(id, &rec, loop.LoopEvent{Kind: loop.EventSpecReady}, now); err != nil {
				return work, err
			}
			work = true
			item, err = d.db.Load(id)
			if err != nil {
				return work, err
			}
		case workflow.StateImplementing, workflow.StateReviewing:
			// Human drove the spec flow manually; adapt without rerunning product.
			if _, err := d.feed(id, &rec, loop.LoopEvent{Kind: loop.EventSpecReady}, now); err != nil {
				return work, err
			}
			work = true
		default:
			return work, nil
		}
	}

	if rec.Loop.Stage == loop.StageImplementing {
		if item.State == workflow.StateReady {
			if err := d.applyWorkflow(id, workflow.CommandBeginImplementation, "", now); err != nil {
				return work, err
			}
			work = true
			item, err = d.db.Load(id)
			if err != nil {
				return work, err
			}
		}
		if item.State != workflow.StateImplementing && item.State != workflow.StateReviewing {
			return work, nil
		}
		outcome, err := d.ExecuteRole(ctx, id, rec, projectID, submodulePath, manifest, "implementer", now)
		if err != nil {
			return work, err
		}
		work = true
		var event loop.LoopEvent
		if outcome.FailureSignature != "" {
			event = loop.LoopEvent{Kind: loop.EventGatesFailed, Signature: outcome.FailureSignature}
		} else {
			if item.State == workflow.StateImplementing {
				if err := d.applyWorkflow(id, workflow.CommandSubmitReview, "", now); err != nil {
					return work, err
				}
				item, err = d.db.Load(id)
				if err != nil {
					return work, err
				}
			}
			event = loop.LoopEvent{Kind: loop.EventGatesPassed}
		}
		decision, err := d.feed(id, &rec, event, now)
		if err != nil {
			return work, err
		}
		if decision.Blocked {
			return d.mirrorBlock(id, item, rec.Loop.BlockedReason, now)
		}
		return work, nil
	}

	if rec.Loop.Stage == loop.StageReviewing {
		if item.State == workflow.StateImplementing {
			// Catch-up: submit landed but the gates feed did not; re-feed without re-gating.
			if _, err := d.feed(id, &rec, loop.LoopEvent{Kind: loop.EventGatesPassed}, now); err != nil {
				return work, err
			}
			work = true
		}
		if item.State != workflow.StateReviewing {
			return work, nil
		}
		outcome, err := d.ExecuteRole(ctx, id, rec, projectID, submodulePath, manifest, "reviewer", now)
		if err != nil {
			return work, err
		}
		work = true
		event := loop.LoopEvent{Kind: loop.EventReviewed, Verdict: loop.VerdictApprove}
		if !outcome.Approved {
			event.Verdict = loop.VerdictChanges
			event.Signature = outcome.FailureSignature
		}
		decision, err := d.feed(id, &rec, event, now)
		if err != nil {
			return work, err
		}
		if decision.Blocked {
			return d.mirrorBlock(id, item, rec.Loop.BlockedReason, now)
		}
		if rec.Loop.Stage == loop.StageDone {
			closed, err := d.closeIfPresent(projectID, submodulePath)
			if err != nil {
				return work, err
			}
			return work || closed, nil
		}
		return work, nil
	}

	return work, nil
}

func sortedIDs(ids map[workflow.WorkItemID]bool) []workflow.WorkItemID {
	out := make([]workflow.WorkItemID, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
