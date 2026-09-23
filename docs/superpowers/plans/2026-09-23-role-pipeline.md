# Role Execution Pipeline Implementation Plan (Increment 18)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One tick carries a Work Item through prepare → thread → gates → evidence →
loop advance: intake submits, automation triages, product writes the Spec, the
implementer runs under gates, the reviewer approves, terminals close worktrees, and
blocks mirror into the workflow — with loop state persisted across restarts and no
duplicate commands.

**Architecture:** `daemon` owns orchestration only — project resolution via `registry`,
worktrees via `workspace`, routing via `telemetry`, threads via `codex`, gates via
`stack`, evidence via `delivery`, authorization via `gatekeeper`, persistence via
`journal` plus its own loop-record file. It owns no policy. All other modules stay
untouched with no new imports. MCP, GitHub polling, and human-surface wiring arrive in
later increments; the tick still consumes the `IntakeSource` interface.

**Tech Stack:** Go 1.27.1 standard library plus `internal/journal`, `internal/workflow`,
`internal/gatekeeper`, `internal/dashboard`, `internal/registry`, `internal/workspace`,
`internal/telemetry`, `internal/codex`, `internal/stack`, `internal/loop`,
`internal/delivery`, and `gopkg.in/yaml.v3`; stub `codex`/toolchain binaries via `PATH`
in tests, table-driven tests.

**Spec:** `docs/superpowers/plans/2026-09-22-harness-organism.md` (Increment 18),
`docs/workflow.md` (development loop rules 2–8, Human Gates), `docs/security.md`
(external prose is data; credentials outside worktrees).

## Global Constraints

- Use Go 1.27.1. Standard library plus the listed internal modules and `gopkg.in/yaml.v3`
  only.
- Use the canonical terms from `CONTEXT.md` (Work Item, Agent Role, Agent Thread,
  Project Worktree, Execution, Gate, Evidence, Human Gate); never write session, job,
  task, or ticket.
- Gates run on host toolchains through `stack.Run` in this increment. Containerized
  gates via `runner` plug in when the image catalog lands (deferred with rationale:
  no image mapping exists yet, and the pipeline decisions under test are identical
  either way). The organism master plan's `runner` mention is corrected in Task 4.
- `Tick` takes a context (`Tick(ctx, now)`) so `Run` cancellation interrupts long
  threads; update the Increment 17 call sites mechanically in the same commit.
- Every journal mutation is preceded by a Gatekeeper allow and followed by loop-record
  persistence; workflow commands are applied before the corresponding loop feed so a
  crash between them resumes consistently (workflow ahead, loop behind — never the reverse).
- Loop records persist to `<workspace>/.harness/daemon-loops.json` (bounded, validated);
  fix-cycle accounting and spent totals survive restarts in that file, never in memory alone.
- Terminal loop states close the worktree only if its directory still exists (idempotent);
  commits survive in the submodule object store.
- Loop `blocked` mirrors into a workflow `block` carrying the loop's `BlockedReason`;
  loop `done` via reviewer approval issues no workflow command (the Human Gate owns
  `approve_pr` through human surfaces).
- Events carry `CostUSD: 0` with rationale recorded: token-to-USD rate tables arrive with
  real billing; the loop budget gate stays structurally enforced and is covered at the
  `loop` level (Increment 8).
- Human-driven workflow jumps ahead of the loop are caught up, never fought: reviewing
  work with an implementing loop feeds `gates_passed`; specifying/blocked/failed and
  post-review states are left to human surfaces.
- Tests use stub `codex`/toolchain binaries via `PATH` plus real git/submodule fixtures;
  no daemon, model, registry, or network calls.
- Do not add generic repository, provider, manager, service, utils, or common packages;
  new code lives in `internal/daemon` only.

---

### Task 1: Extend config and persist loop records

**Files:**
- Modify: `internal/daemon/config.go`, `internal/daemon/config_test.go`
- Create: `internal/daemon/loops.go`
- Test: `internal/daemon/loops_test.go`

**Interfaces:**
- Consumes: `Config` from Increment 17; `loop.LoopState`.
- Produces: `Config{RequiredRoles, BudgetUSD, MaxDuration}` with defaults
  (`[product implementer reviewer]`, `10`, `2h`); `LoopRecord struct`;
  `(Daemon).loadLoops/persistLoops` (unexported, exercised through record helpers);
  exported `LoopState(id) (loop.LoopState, bool)` for tests.

- [ ] **Step 1: Write the failing config/record test**

```go
package daemon

import (
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/loop"
)

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg, err := LoadConfig(writeConfig(t, root, validBody(root)))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RequiredRoles) != 3 || cfg.BudgetUSD != 10 || cfg.MaxDuration != 2*time.Hour {
		t.Fatalf("defaults missing: %#v", cfg)
	}
}

func TestConfigCustomLoopPolicy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	body := "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\nroles: [implementer]\nbudgetUSD: 5\nmaxDuration: 30m\n"
	cfg, err := LoadConfig(writeConfig(t, root, body))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RequiredRoles) != 1 || cfg.BudgetUSD != 5 || cfg.MaxDuration != 30*time.Minute {
		t.Fatalf("custom policy lost: %#v", cfg)
	}
}

func TestLoopRecordRoundTrip(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	rec := LoopRecord{Title: "First", ProjectID: "demo"}
	state, err := loop.Begin("owner/repo#1", []string{"product", "implementer", "reviewer"}, 10, 2*time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rec.Loop = state
	if err := d.saveLoop("owner/repo#1", rec); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	got, ok := reopened.LoopState("owner/repo#1")
	if !ok || got.Stage != loop.StageSpecifying || got.BudgetUSD != 10 {
		t.Fatalf("record lost: %#v %v", got, ok)
	}
}
```

NOTE: `fakeSource`, `testConfig` live in `daemon_test.go` (same package) — reuse them.
`saveLoop` persists immediately (write-through); add `LoopState(id)` accessor returning
the loop half of the record.

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/daemon -run 'TestConfigDefaults|TestConfigCustomLoopPolicy|TestLoopRecordRoundTrip' -count=1`

Expected: FAIL (no `RequiredRoles`/`BudgetUSD`/`MaxDuration` fields, no `LoopRecord`).

- [ ] **Step 3: Implement config policy and record persistence**

Config additions in `config.go`: yaml fields `roles`, `budgetUSD`, `maxDuration`
(string, parsed like `pollInterval`); defaults applied when absent; validation
(non-empty roles, positive budget and duration) folded into `Validate`.

```go
// internal/daemon/loops.go
package daemon

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/marceloamoreno87/workflow-dev-template/internal/loop"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

const loopsFileName = "daemon-loops.json"

const maxLoopRecords = 100000

type LoopRecord struct {
	Loop        loop.LoopState
	Title       string
	ProjectID   string
	ProductDone bool
}

func (d *Daemon) loopsPath() string {
	return harnessDir(d.cfg.WorkspaceRoot) + "/daemon-loops.json"
	// NOTE: harnessDir joins root + ".harness"; filepath.Join when writing the file.
}

func (d *Daemon) loadLoops() error { /* like loadKnown: bounded read, unmarshal map[string]LoopRecord, validate ids + cap */ }

func (d *Daemon) persistLoops() error { /* marshal map, write 0o644 */ }

func (d *Daemon) saveLoop(id workflow.WorkItemID, rec LoopRecord) error {
	if d.loops == nil {
		d.loops = map[workflow.WorkItemID]LoopRecord{}
	}
	d.loops[id] = rec
	return d.persistLoops()
}

func (d *Daemon) LoopState(id workflow.WorkItemID) (loop.LoopState, bool) {
	rec, ok := d.loops[id]
	return rec.Loop, ok
}
```

Wire `loops` map + `loadLoops()` into `Open` (fail closed like `loadKnown`).
Use `filepath.Join` for the path (fix the sketch's string concat when writing the file).

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/daemon -count=1`

Expected: PASS (existing config tests still pass: absent fields take defaults).

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/daemon/ && go test ./...`

Expected: PASS.

```bash
git add internal/daemon/config.go internal/daemon/config_test.go internal/daemon/loops.go internal/daemon/loops_test.go
git commit -m "feat(daemon): loop policy config and record persistence"
```

### Task 2: Resolve projects and advance the (workflow, loop) machine

**Files:**
- Create: `internal/daemon/advance.go`
- Test: `internal/daemon/advance_test.go` (+ shared git fixture helper file `internal/daemon/fixture_test.go`)
- Modify: `internal/daemon/daemon.go` (`Tick(ctx, now)`, advance loop over sorted known ids, continue-on-first-error), `internal/daemon/daemon_test.go` + `run_test.go` + `flow_test.go` (mechanical `Tick` signature update + behavior updates, see below)

**Interfaces:**
- Consumes: `registry` (registry.yaml → project path + repo), manifest (classification +
  adapter), `loop` machine, `workflow` commands via gatekeeper+journal.
- Produces: `(Daemon).advanceItem(ctx, id, now) (bool, error)` implementing the
  (workflow-state, loop-record) table; `resolveProject(id) (projectID, submodulePath,
  manifest, error)`.

The advance table (W = journal state, L = loop record or absent):

- `inbox` → apply `begin_triage`. (No loop yet.)
- `triage` → nothing (Human Gate owns authorize/reject).
- `ready`, no record → `Begin` + persist; run product (Task 3 `ExecuteRole`);
  on success mark `ProductDone`, feed `spec_ready`; if Next is implementer and W is
  still `ready`, apply `begin_implementation`; run implementer leg (Task 3).
- `ready`, L=`specifying` without `ProductDone` → same as fresh (crash-safe rerun).
- `ready`/`implementing`, L=`implementing` → run implementer leg; on pass apply
  `submit_review` (only if W is `implementing`), feed `gates_passed`; on fail feed
  `gates_failed`; on loop-blocked apply workflow `block` with `BlockedReason`.
- `implementing`, no record → `Begin` + feed `spec_ready` (human drove workflow
  directly; loop adapts without rerunning product), then implementer leg.
- `reviewing`, L=`reviewing` → run reviewer leg (Task 3); approve closes worktree.
- `reviewing`, L=`implementing` → feed `gates_passed` (human-override catch-up),
  then reviewer leg.
- L terminal (`done`) → close worktree if present, nothing else.
- L=`blocked` with W advanceable → apply workflow `block` with `BlockedReason`.
- `specifying`, `blocked`, `failed`, post-review workflow states → skip (human surfaces own them).

`resolveProject`: repo = substring before the last `#` in the Work Item id; read
`<root>/.harness/registry.yaml` → `ParseRegistry` + `Validate` → match
`Github.Repository`; read `<sub>/.harness/project.yaml` → `RejectSecrets` +
`ParseProjectManifest` + `Validate`. No match or bad manifest → skip item this tick
(`nil` work, `nil` error — surfacing unregistered items arrives later).

Existing-test updates (behavior intentionally extended this increment):
- `TestTickSubmitsUnknownItems`: first tick now submits AND triages (expect
  `triage`/version 2); second tick still a no-op.
- `TestTickSkipsJournaledItems` → rename to `TestTickPicksUpSeededItems`: seeded inbox
  item advances to triage.
- `TestRestartRecoversCursor`: reopened tick advances the inbox item to triage
  (didWork true, journal shows triage); a further tick is the no-op.
- `run_test.go` / `flow_test.go`: mechanical `Tick(ctx, …)` updates.

- [ ] **Step 1: Write the fixture helper and the advance test**

`fixture_test.go` (shared): build outer git repo + inner repo + submodule
`projects/demo` (with `-c protocol.file.allow=always`), registry.yaml, manifest,
`stubBin` helper writing executable stub scripts, `stubCodex` (mode file: `ok` →
completed JSON), PATH+CODEX_HOME wiring via `t.Setenv` (no `t.Parallel` in tests
using it).

`advance_test.go`: `TestAutoTriage` (inbox→triage in one tick, automation actor in
journal events), `TestTriageWaitsForHuman` (second tick no-op), `TestUnregisteredSkips`
(item for unknown repo: no error, no journal entry).

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/daemon -run 'TestAutoTriage|TestTriageWaits|TestUnregisteredSkips' -count=1`

Expected: FAIL (`advanceItem`, `resolveProject` undefined; behavior absent).

- [ ] **Step 3: Implement resolution and the advance table (product/implementer/reviewer
  legs call `ExecuteRole`, which lands in Task 3 — stub the call sites to return
  "not implemented" errors in this task is forbidden; instead implement the table
  fully and let Task 3 provide `ExecuteRole`. Order within the task: write `advance.go`
  with the table, watch tests fail on missing `ExecuteRole`, then implement Task 3
  before running green. The Task 2 commit happens after Task 3 lands if needed —
  prefer: commit Task 2 table + Task 3 execution together only if inseparable; otherwise
  keep commits per task.)

Simplification to keep tasks committable: Task 2 implements the table with the
implementer/reviewer/product legs behind a small `runRole` function variable defaulting
to `ExecuteRole` (real, Task 3) — no: function variables complicate. Decision: Task 2
writes `advance.go` + tests that only exercise triage/wait/unregistered paths (no role
execution), which pass without `ExecuteRole`; Task 3 adds role legs + `ExecuteRole`
with the remaining tests. The table rows for role legs are marked `// Task 3` and return
`fmt.Errorf("%w: role execution not wired", ErrDaemon)` until then — no wait, that
would break Task 2's own compile? No: returning an error needs no new symbols. But a
test hitting those rows would fail... Task 2 tests avoid those rows. Acceptable and
honest: each commit green on its own paths.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/daemon -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/daemon/ && go test ./...`

Expected: PASS.

```bash
git add internal/daemon/advance.go internal/daemon/advance_test.go internal/daemon/fixture_test.go internal/daemon/daemon.go internal/daemon/daemon_test.go internal/daemon/run.go internal/daemon/run_test.go internal/daemon/flow_test.go
git commit -m "feat(daemon): resolve projects and advance triage"
```

### Task 3: Execute roles end to end

**Files:**
- Create: `internal/daemon/execute.go`
- Test: `internal/daemon/execute_test.go`

**Interfaces:**
- Consumes: `workspace` (ensure-or-prepare), `telemetry` (route + usage),
  `codex` (thread), `stack` (host gates), `delivery` (evidence), `loop` (role mapping).
- Produces: `EvidenceBundle struct`, `(Daemon).ExecuteRole(ctx, id, role, now)
  (EvidenceBundle, error)`, `ensureWorktree`, role→(taskKind, criteria) map.

`ExecuteRole` steps for `(id, role)` with the persisted record + resolved project:

1. `ensureWorktree`: stat `<root>/.worktrees/<project>`; absent → `workspace.Prepare`;
   present → `workspace.WriteGitConfig` refresh; return `workspace.Prepared`.
2. `telemetry.Select(taskKind[role], manifest.Classification)`; build `codex.ThreadSpec`
   (objective from record title or `Work on <id>`, SpecRef `<id>`, model/effort from
   route, workdir = prepared dir, `GitConfigGlobal` = prepared config, timeout 30m,
   deadline `now+30m`, budget = loop budget, criteria per role).
3. `codex.Run` → on transport/timeout/validation error return it (no loop event;
   next tick retries).
4. Product role: status `completed` → mark `ProductDone`, persist, return empty bundle.
   Other statuses → return `fmt.Errorf("%w: product thread %s", ErrDaemon, status)`
   (next tick retries; no infinite silent loop — the error surfaces in tick errors).
5. Implementer/reviewer: for each gate of the resolved stack adapter, `stack.Run`
   with rendered `GateCommand` argv in the prepared dir; collect `delivery.Evidence`
   per gate (`CommandID daemon-<id>-<gate>`, `Tool stack:<binary>`, `Version unpinned`
   with rationale comment, window from run, exit code, commit = best-effort worktree
   HEAD via `git rev-parse`, empty on failure) and `Validate` each (validation failure
   is a daemon bug → return error).
6. Usage: `telemetry.UsageRecord{WorkItem: id, Model, tokens, CostUSD: 0, Duration}`
   validated (rate-table rationale comment); bundle = `{Evidence, Usage}`.
7. Verdict mapping for reviewer: `completed` → approve path (caller feeds approve);
   `blocked`/`failed` → changes with signature `"review:"+status`. Return a small
   `RoleOutcome{Approved bool, FailureSignature string}` alongside the bundle so
   `advanceItem` feeds the right loop event: signature `RoleOutcome` struct
   `{Bundle EvidenceBundle, Approved bool, FailureSignature string, ProductDone bool}`.

`advanceItem` role legs (filling Task 2's marked rows): product leg uses
`ProductDone`; implementer leg uses gate-failure signatures (sorted failed gate names
joined by `,`; empty when all pass); reviewer leg uses the verdict mapping; loop
`done` closes the worktree (guarded by `os.Stat`); loop `blocked` applies the workflow
`block` when W allows it.

- [ ] **Step 1: Write the failing execution test**

`execute_test.go`: `TestImplementerPassesGates` (stub codex ok + stub `go`/`gofmt` exit 0
→ bundle with 3 validated evidence entries for go-service, usage model set),
`TestImplementerGateFailure` (stub `go` exit 1 → outcome signature `"test"`,
no error), `TestReviewerMapsBlockedToChanges` (stub codex mode `blocked` →
`Approved=false`, signature `"review:blocked"`), `TestTerminalClosesWorktree`
(`Close` after simulated done removes the dir; second close is a no-op).

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/daemon -run 'TestImplementer|TestReviewerMaps|TestTerminalCloses' -count=1`

Expected: FAIL (`ExecuteRole`, `EvidenceBundle`, `RoleOutcome` undefined).

- [ ] **Step 3: Implement `ExecuteRole` and fill the advance legs**

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/daemon -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/daemon/ && go test ./...`

Expected: PASS.

```bash
git add internal/daemon/execute.go internal/daemon/execute_test.go internal/daemon/advance.go
git commit -m "feat(daemon): execute roles end to end"
```

### Task 4: Prove bounds, restart safety, and the full pipeline

**Files:**
- Create: `internal/daemon/pipeline_test.go`
- Modify: `docs/implementation-plan.md`, `docs/superpowers/plans/2026-09-22-harness-organism.md`
  (correct the `runner` line: host gates via `stack` until the image catalog lands).

**Interfaces:**
- Consumes: complete pipeline from Tasks 1-3.
- Produces: executable evidence of happy-path ticks (inbox→…→reviewing with loop done
  and worktree closed), repeated-failure blocking mirrored into workflow `blocked` with
  the loop reason, and restart mid-pipeline without duplicate commands.

- [ ] **Step 1: Add the pipeline test**

`pipeline_test.go`: `TestHappyPathPipeline` (ticks with human authorize applied
directly to the journal between ticks to simulate surfaces: inbox→(tick)→triage→
(authorize)→ready→(tick: product+implement+gates+submit_review)→reviewing with loop
reviewing→(tick: reviewer approve)→loop done, worktree closed, journal holds
`submit_review`, no `approve_pr`); `TestRepeatedGateFailureBlocks` (stub `go` exit 1
twice with same gate → second feed blocks loop with `repeated failure`, workflow
`blocked` carrying that reason); `TestRestartMidPipeline` (close/reopen after the
implement tick → next tick performs no duplicate `begin_implementation`: journal event
count for the item unchanged, loop record intact, reviewer leg proceeds).

Counting journal events per item requires reading the store: `journal` has no list API
— derive counts by re-`Load` versions (`Load` returns folded item with `Version` =
event count). Assert versions, not raw events.

- [ ] **Step 2: Run the pipeline test**

Run: `go test ./internal/daemon -run 'TestHappyPathPipeline|TestRepeatedGateFailureBlocks|TestRestartMidPipeline' -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean builds of `cmd/harness` and `cmd/harnessd`.

- [ ] **Step 4: Mark Increment 18 verified and commit**

In `docs/implementation-plan.md`, add Increment 18 to the v2 plan index and a
verification checklist. In the organism master plan, correct Increment 18's wiring
line to host gates via `stack` (runner when the image catalog lands) with the
rationale. Do not mark it complete until the commands above pass on master.

```bash
git add internal/daemon/pipeline_test.go docs/implementation-plan.md docs/superpowers/plans/2026-09-22-harness-organism.md
git commit -m "test(daemon): verify role execution pipeline"
```
