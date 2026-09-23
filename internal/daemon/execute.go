// internal/daemon/execute.go
package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/codex"
	"github.com/marceloamoreno87/workflow-dev-template/internal/delivery"
	"github.com/marceloamoreno87/workflow-dev-template/internal/registry"
	"github.com/marceloamoreno87/workflow-dev-template/internal/stack"
	"github.com/marceloamoreno87/workflow-dev-template/internal/telemetry"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workspace"
)

const threadTimeout = 30 * time.Minute

var roleTaskKinds = map[string]string{
	"product":     "spec",
	"implementer": "implement",
	"reviewer":    "review",
}

var roleCriteria = map[string][]string{
	"product":     {"spec complete, no open questions"},
	"implementer": {"declared gates pass"},
	"reviewer":    {"verdict recorded"},
}

type EvidenceBundle struct {
	Evidence []delivery.Evidence
	Usage    telemetry.UsageRecord
}

type RoleOutcome struct {
	Bundle           EvidenceBundle
	Approved         bool
	FailureSignature string
	ProductDone      bool
}

func (d *Daemon) ensureWorktree(projectID, submodulePath string) (workspace.Prepared, error) {
	dir := filepath.Join(d.cfg.WorkspaceRoot, ".worktrees", projectID)
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return workspace.Prepared{}, err
		}
		return workspace.Prepare(d.cfg.WorkspaceRoot, projectID, submodulePath)
	}
	return workspace.WriteGitConfig(dir)
}

func (d *Daemon) ExecuteRole(ctx context.Context, id workflow.WorkItemID, rec LoopRecord, projectID, submodulePath string, manifest registry.ProjectManifest, role string, now time.Time) (RoleOutcome, error) {
	taskKind, ok := roleTaskKinds[role]
	if !ok {
		return RoleOutcome{}, fmt.Errorf("%w: unknown role %q", ErrDaemon, role)
	}
	prepared, err := d.ensureWorktree(projectID, submodulePath)
	if err != nil {
		return RoleOutcome{}, err
	}
	route, err := telemetry.Select(taskKind, manifest.Data.Classification)
	if err != nil {
		return RoleOutcome{}, err
	}
	objective := strings.TrimSpace(rec.Title)
	if objective == "" {
		objective = "Work on " + string(id)
	}
	spec := codex.ThreadSpec{
		Role:               role,
		Objective:          objective,
		SpecRef:            string(id),
		Model:              route.Model,
		Effort:             route.Effort,
		Classification:     manifest.Data.Classification,
		WorkspaceRoot:      d.cfg.WorkspaceRoot,
		Workdir:            prepared.Dir,
		BudgetUSD:          rec.Loop.BudgetUSD,
		Deadline:           now.Add(threadTimeout),
		CompletionCriteria: roleCriteria[role],
		Timeout:            threadTimeout,
		GitConfigGlobal:    prepared.ConfigFile,
	}
	threadCtx, cancel := context.WithTimeout(ctx, threadTimeout)
	defer cancel()
	result, err := codex.Run(threadCtx, spec)
	if err != nil {
		return RoleOutcome{}, err
	}
	usage := telemetry.UsageRecord{
		WorkItem:     string(id),
		Model:        route.Model,
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		// CostUSD stays zero: token-to-USD rate tables arrive with real billing.
		// The loop budget gate stays structurally enforced (Increment 8).
		Duration: result.Duration,
	}
	if err := usage.Validate(); err != nil {
		return RoleOutcome{}, err
	}
	if role == "product" {
		if result.Status != "completed" {
			return RoleOutcome{}, fmt.Errorf("%w: product thread %s", ErrDaemon, result.Status)
		}
		return RoleOutcome{Bundle: EvidenceBundle{Usage: usage}, ProductDone: true}, nil
	}
	adapter, err := stack.Resolve(manifest.Stack.Adapter)
	if err != nil {
		return RoleOutcome{}, err
	}
	var evidence []delivery.Evidence
	var failed []string
	for _, gate := range sortedGates(adapter) {
		argv, cwd, err := stack.GateCommand(adapter, gate, prepared.Dir)
		if err != nil {
			return RoleOutcome{}, err
		}
		run, err := stack.Run(threadCtx, argv, cwd, adapter.Gates[gate].Timeout)
		entry := delivery.Evidence{
			CommandID: fmt.Sprintf("daemon-%s-%s", id, gate),
			Tool:      "stack:" + argv[0],
			// Pinned tool versions arrive with the container image catalog;
			// until then the toolchain is whatever the host provides.
			Version:       "unpinned",
			StartedAt:     now,
			FinishedAt:    now.Add(run.Duration),
			ExitCode:      run.ExitCode,
			Commit:        worktreeHead(prepared.Dir),
			Applicable:    true,
			Deterministic: true,
		}
		if err != nil {
			failed = append(failed, gate)
		}
		if verr := entry.Validate(); verr != nil {
			return RoleOutcome{}, verr
		}
		evidence = append(evidence, entry)
	}
	bundle := EvidenceBundle{Evidence: evidence, Usage: usage}
	if role == "reviewer" {
		if result.Status == "completed" {
			return RoleOutcome{Bundle: bundle, Approved: true}, nil
		}
		return RoleOutcome{Bundle: bundle, FailureSignature: "review:" + result.Status}, nil
	}
	if len(failed) > 0 {
		return RoleOutcome{Bundle: bundle, FailureSignature: strings.Join(failed, ",")}, nil
	}
	return RoleOutcome{Bundle: bundle, Approved: true}, nil
}

func sortedGates(a stack.Adapter) []string {
	names := make([]string, 0, len(a.Gates))
	for name := range a.Gates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// worktreeHead returns the worktree HEAD sha, best effort: evidence without a
// commit is still valid, so failures degrade to "" instead of failing the gate.
func worktreeHead(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := stack.Run(ctx, []string{"git", "rev-parse", "HEAD"}, dir, 10*time.Second)
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(res.Output)
}
