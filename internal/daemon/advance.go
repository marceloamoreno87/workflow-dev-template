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
	"github.com/marceloamoreno87/workflow-dev-template/internal/registry"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
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
	_ = ctx // Task 3 consumes it for role execution.
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
		_, _, _, found, err := d.resolveProject(id)
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}
		return false, fmt.Errorf("%w: role execution not wired", ErrDaemon)
	default:
		return false, nil
	}
}

func sortedIDs(ids map[workflow.WorkItemID]bool) []workflow.WorkItemID {
	out := make([]workflow.WorkItemID, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
