# Workflow Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a deterministic Go domain core in which authenticated Commands are authorized by Gatekeeper, produce immutable Events, and rebuild Work Item State without infrastructure dependencies.

**Architecture:** `workflow` owns state transitions and event folding; `gatekeeper` owns Actor, Quality Profile, and Human Gate authorization. Both are pure Modules with no SQLite, GitHub, Docker, Codex, or network imports. The next increment will persist the same Command and Event values without changing these interfaces.

**Tech Stack:** Go 1.27.1 standard library, table-driven tests, fuzz tests

**Spec:** `docs/architecture.md`, `docs/workflow.md`, `docs/contracts.md`

## Global Constraints

- Use Go 1.27.1, the latest stable patch verified from the official Go release history on 2026-09-20.
- Keep `workflow` and `gatekeeper` free of I/O and third-party dependencies.
- Use the canonical terms and meanings from `CONTEXT.md`.
- A Command is idempotent by `CommandID` at the future journal seam; this increment must preserve that field but must not invent persistence.
- Every accepted Command emits at least one Event; every rejected Command emits none.
- State changes occur only by folding Events.
- Do not add generic repository, provider, manager, service, or utility packages.

---

### Task 1: Bootstrap the Go module and workflow value types

**Files:**
- Create: `go.mod`
- Create: `internal/workflow/types.go`
- Test: `internal/workflow/types_test.go`

**Interfaces:**
- Produces: `WorkItemID`, `CommandID`, `ActorID`, `Version`, `State`, `CommandType`, `EventType`, and their `Valid() bool` methods.
- Consumes: nothing.

- [ ] **Step 1: Write the failing type-validation test**

```go
package workflow

import "testing"

func TestStateValid(t *testing.T) {
	t.Parallel()

	for _, state := range []State{
		StateInbox, StateTriage, StateReady, StateSpecifying,
		StateImplementing, StateReviewing, StateChangesRequested,
		StateReadyToDeploy, StateDeploying, StateClientQA,
		StateRollingOut, StateDone, StateBlocked, StateFailed,
		StateCancelled,
	} {
		if !state.Valid() {
			t.Fatalf("expected %q to be valid", state)
		}
	}

	if State("invented").Valid() {
		t.Fatal("unexpected valid invented state")
	}
}
```

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/workflow -run TestStateValid -count=1`

Expected: FAIL because `State` and its constants are undefined.

- [ ] **Step 3: Create the module and minimal types**

```go
// go.mod
module github.com/marceloamoreno87/workflow-dev-template

go 1.27.1
```

```go
// internal/workflow/types.go
package workflow

type WorkItemID string
type CommandID string
type ActorID string
type Version uint64

type State string

const (
	StateInbox            State = "inbox"
	StateTriage           State = "triage"
	StateReady            State = "ready"
	StateSpecifying       State = "specifying"
	StateImplementing     State = "implementing"
	StateReviewing        State = "reviewing"
	StateChangesRequested State = "changes_requested"
	StateReadyToDeploy    State = "ready_to_deploy"
	StateDeploying        State = "deploying"
	StateClientQA         State = "client_qa"
	StateRollingOut       State = "rolling_out"
	StateDone             State = "done"
	StateBlocked          State = "blocked"
	StateFailed           State = "failed"
	StateCancelled        State = "cancelled"
)

func (s State) Valid() bool {
	switch s {
	case StateInbox, StateTriage, StateReady, StateSpecifying,
		StateImplementing, StateReviewing, StateChangesRequested,
		StateReadyToDeploy, StateDeploying, StateClientQA,
		StateRollingOut, StateDone, StateBlocked, StateFailed,
		StateCancelled:
		return true
	default:
		return false
	}
}

type CommandType string
type EventType string
```

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/workflow -run TestStateValid -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/workflow/types.go internal/workflow/types_test.go && go test ./...`

Expected: PASS.

```bash
git add go.mod internal/workflow/types.go internal/workflow/types_test.go
git commit -m "feat(workflow): define core value types"
```

### Task 2: Rebuild Work Item State exclusively from Events

**Files:**
- Create: `internal/workflow/event.go`
- Create: `internal/workflow/state.go`
- Test: `internal/workflow/state_test.go`

**Interfaces:**
- Consumes: `WorkItemID`, `ActorID`, `CommandID`, `State`, and `Version` from Task 1.
- Produces: `Event`, `WorkItem`, and `Fold(events []Event) (WorkItem, error)`.

- [ ] **Step 1: Write the failing fold test**

```go
package workflow

import (
	"testing"
	"time"
)

func TestFoldRebuildsStateAndVersion(t *testing.T) {
	t.Parallel()

	events := []Event{
		{ID: "e1", AggregateID: "repo#1", Version: 1, Type: EventWorkSubmitted, To: StateInbox, At: time.Unix(1, 0)},
		{ID: "e2", AggregateID: "repo#1", Version: 2, Type: EventStateChanged, From: StateInbox, To: StateTriage, At: time.Unix(2, 0)},
	}

	got, err := Fold(events)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "repo#1" || got.State != StateTriage || got.Version != 2 {
		t.Fatalf("unexpected aggregate: %#v", got)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/workflow -run TestFoldRebuildsStateAndVersion -count=1`

Expected: FAIL because `Event`, `WorkItem`, and `Fold` are undefined.

- [ ] **Step 3: Implement immutable event values and strict folding**

Define `EventWorkSubmitted` and `EventStateChanged`. Define `Event` with `ID string`, `AggregateID WorkItemID`, `Version Version`, `CommandID CommandID`, `ActorID ActorID`, `Type EventType`, `From State`, `To State`, `Reason string`, and `At time.Time`.

Define `WorkItem` with `ID WorkItemID`, `State State`, `ResumeState State`, and `Version Version`.

Implement `Fold` with these exact invariants:

```go
var (
	ErrEmptyHistory      = errors.New("empty event history")
	ErrAggregateMismatch = errors.New("event aggregate mismatch")
	ErrVersionGap        = errors.New("event version gap")
	ErrTransitionMismatch = errors.New("event transition mismatch")
)
```

- the first event must be `EventWorkSubmitted`, version 1, with valid `To`;
- every later event must match the first Aggregate ID;
- versions must increase by exactly one;
- `EventStateChanged.From` must equal current State;
- entering `blocked` stores the prior State in `ResumeState`;
- leaving `blocked` clears `ResumeState`;
- unknown Event types return an error.

- [ ] **Step 4: Add invariant tests**

Add table cases for aggregate mismatch, version gap, transition mismatch, unknown event type, and blocked/resumed state.

- [ ] **Step 5: Run, format, and commit**

Run: `gofmt -w internal/workflow/event.go internal/workflow/state.go internal/workflow/state_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/workflow/event.go internal/workflow/state.go internal/workflow/state_test.go
git commit -m "feat(workflow): fold immutable work item events"
```

### Task 3: Turn valid Commands into Events

**Files:**
- Create: `internal/workflow/command.go`
- Create: `internal/workflow/workflow.go`
- Test: `internal/workflow/workflow_test.go`

**Interfaces:**
- Consumes: `WorkItem`, `State`, and `Event` from Tasks 1-2.
- Produces: `Command`, `Workflow.Handle(now time.Time, item WorkItem, command Command) ([]Event, error)`, and `Workflow.Allowed(item WorkItem) []CommandType`.

- [ ] **Step 1: Write a failing transition test**

```go
package workflow

import (
	"testing"
	"time"
)

func TestHandleBeginTriage(t *testing.T) {
	t.Parallel()

	w := Workflow{}
	item := WorkItem{ID: "repo#1", State: StateInbox, Version: 1}
	command := Command{ID: "c2", AggregateID: item.ID, ExpectedVersion: 1, ActorID: "operator", Type: CommandBeginTriage}

	events, err := w.Handle(time.Unix(2, 0), item, command)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].From != StateInbox || events[0].To != StateTriage || events[0].Version != 2 {
		t.Fatalf("unexpected events: %#v", events)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/workflow -run TestHandleBeginTriage -count=1`

Expected: FAIL because Command and Workflow are undefined.

- [ ] **Step 3: Implement Command validation and transition table**

Define `Command` with `ID`, `AggregateID`, `ExpectedVersion`, `ActorID`, `Type`, and `Reason`. Reject empty IDs, mismatched Aggregate IDs, stale versions, terminal states, and blank reasons for block/cancel/reject/fail Commands.

Implement this transition table exactly:

```go
var transitions = map[State]map[CommandType]State{
	StateInbox: {CommandBeginTriage: StateTriage, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateTriage: {CommandAuthorizeWork: StateReady, CommandRejectWork: StateCancelled, CommandBlock: StateBlocked},
	StateReady: {CommandBeginSpec: StateSpecifying, CommandBeginImplementation: StateImplementing, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateSpecifying: {CommandApproveSpec: StateReady, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateImplementing: {CommandSubmitReview: StateReviewing, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateReviewing: {CommandRequestChanges: StateChangesRequested, CommandApprovePR: StateReadyToDeploy, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateChangesRequested: {CommandBeginImplementation: StateImplementing, CommandCancel: StateCancelled},
	StateReadyToDeploy: {CommandBeginDeploy: StateDeploying, CommandBlock: StateBlocked, CommandCancel: StateCancelled},
	StateDeploying: {CommandMarkDeploymentHealthy: StateClientQA, CommandFailDeployment: StateFailed},
	StateClientQA: {CommandAcceptFeature: StateRollingOut, CommandRequestChanges: StateChangesRequested, CommandBlock: StateBlocked},
	StateRollingOut: {CommandCompleteRollout: StateDone, CommandBlock: StateBlocked},
	StateFailed: {CommandRetryDeployment: StateReadyToDeploy, CommandCancel: StateCancelled},
}
```

Handle `CommandSubmitWork` only against the zero-value WorkItem and emit `EventWorkSubmitted` at version 1. Handle `CommandResolveBlock` by transitioning from `blocked` to `ResumeState`.

- [ ] **Step 4: Add a table test for every transition and every invalid source**

Build tests by iterating `transitions`. For each entry, assert one `EventStateChanged`, correct version, Actor ID, Command ID, From, and To. For each Command from a different nonterminal State, assert `ErrTransitionNotAllowed` and zero Events.

- [ ] **Step 5: Add `Allowed` tests**

Assert `Allowed` returns sorted Command types for Inbox, Reviewing, Client QA, Blocked, Done, and Cancelled. Terminal states return an empty slice.

- [ ] **Step 6: Run, format, and commit**

Run: `gofmt -w internal/workflow/command.go internal/workflow/workflow.go internal/workflow/workflow_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/workflow/command.go internal/workflow/workflow.go internal/workflow/workflow_test.go
git commit -m "feat(workflow): handle lifecycle commands"
```

### Task 4: Authorize Actions with Gatekeeper

**Files:**
- Create: `internal/gatekeeper/policy.go`
- Test: `internal/gatekeeper/policy_test.go`

**Interfaces:**
- Consumes: `workflow.Command`, `workflow.State`, and `workflow.CommandType`.
- Produces: `Profile`, `ActorKind`, `Decision`, `Context`, and `Policy.Decide(Context, workflow.Command) Decision`.

- [ ] **Step 1: Write the failing client-acceptance authorization test**

```go
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
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/gatekeeper -run TestContributorMayAcceptOnlyDuringClientQA -count=1`

Expected: FAIL because the gatekeeper types are undefined.

- [ ] **Step 3: Implement policy values and deterministic decisions**

Define Profiles `prototype`, `standard`, `critical`; Actors `operator`, `contributor`, `automation`; Decision codes `allowed`, `actor_denied`, `human_gate_required`, `profile_denied`, and `state_denied`.

Rules:

- Operator may request any Workflow-allowed Command.
- Contributor may submit work, accept a Feature, request changes during Client QA, and cancel only their untriaged Inbox Work Item.
- Automation may advance mechanical states but may not authorize work, approve a Spec, approve a PR, accept a Feature, lower a profile, deploy critical work, change policy, or bypass a Gate.
- Standard and critical merge approval require an operator Human Gate.
- Standard production deploy requires an operator Human Gate.
- Critical production deploy, destructive migration, secret/permission change, policy change, and Harness self-change always require an operator Human Gate.
- A lower effective profile than the Project minimum returns `profile_denied`.
- A Command not returned by `workflow.Workflow{}.Allowed` returns `state_denied`.

- [ ] **Step 4: Add a complete decision table test**

Include at least operator merge, contributor acceptance, contributor PR approval denial, automation mechanical transition, automation policy denial, profile lowering, standard deploy, critical deploy, and invalid state.

- [ ] **Step 5: Run, format, and commit**

Run: `gofmt -w internal/gatekeeper/policy.go internal/gatekeeper/policy_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/gatekeeper/policy.go internal/gatekeeper/policy_test.go
git commit -m "feat(gatekeeper): authorize workflow commands"
```

### Task 5: Prove deterministic replay and transition safety

**Files:**
- Create: `internal/workflow/workflow_fuzz_test.go`
- Create: `internal/workflow/scenario_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: the complete Workflow and Gatekeeper interfaces from Tasks 1-4.
- Produces: executable evidence that replay is deterministic and invalid Commands never mutate State.

- [ ] **Step 1: Add the happy-path scenario test**

Drive this exact sequence through Gatekeeper and Workflow, folding after each accepted Command:

```text
SubmitWork → BeginTriage → AuthorizeWork → BeginSpec → ApproveSpec
→ BeginImplementation → SubmitReview → ApprovePR → BeginDeploy
→ MarkDeploymentHealthy → AcceptFeature → CompleteRollout
```

Assert the final State is `done`, the final Version equals the Event count, and replaying a copy of the Events returns an identical WorkItem.

- [ ] **Step 2: Run the scenario test**

Run: `go test ./internal/workflow -run TestHappyPathReplay -count=1`

Expected: PASS.

- [ ] **Step 3: Add a fuzz test for malformed histories**

Use `testing.F` with byte input to generate event versions and From/To states. The property is: `Fold` either returns an error or returns a WorkItem whose Version equals the final Event version and whose State is valid. It must never panic.

- [ ] **Step 4: Run fuzzing for a bounded verification window**

Run: `go test ./internal/workflow -run '^$' -fuzz FuzzFoldNeverReturnsInvalidState -fuzztime 10s`

Expected: PASS with no panic or failing corpus entry.

- [ ] **Step 5: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero test failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

- [ ] **Step 6: Mark Increment 1 verified and commit**

In `docs/implementation-plan.md`, add a checklist below Increment 1 with the commands and evidence required to mark it complete. Do not mark it complete until the commands above pass in the implementation worktree.

```bash
git add internal/workflow/workflow_fuzz_test.go internal/workflow/scenario_test.go docs/implementation-plan.md
git commit -m "test(workflow): verify deterministic lifecycle replay"
```
