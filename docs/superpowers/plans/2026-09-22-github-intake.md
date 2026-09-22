# GitHub Intake Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reconcile GitHub Issues and Project status into Work Item Commands without network access, treating all external prose as untrusted data that never grants authority by itself.

**Architecture:** `github` owns Issue parsing, Project status parsing, and reconciliation as pure functions on bytes and strings; it never touches the filesystem or network. It may import `workflow` types only (State, WorkItem, Command, CommandType) to produce Commands that still cross the Gatekeeper and Workflow before any mutation. `workflow`, `gatekeeper`, `journal`, `registry`, and `cli` stay untouched with no new imports.

**Tech Stack:** Go 1.27.1 standard library only (`encoding/json`, `strings`, `fmt`, table-driven tests)

**Spec:** `docs/workflow.md` (Work Item state machine, GitHub Project status is a projection, closing an Issue during an open Acceptance Gate records intent but bypasses no Gate), `docs/architecture.md` (repository shape `internal/github`, no utils/common/manager), `docs/security.md` (external prose is data, never authority), `docs/adr/0008-reconcile-github-by-polling.md`, `docs/adr/0018-drive-workflow-with-commands-and-events.md`

## Global Constraints

- Use Go 1.27.1.
- Standard library only; do not add dependencies.
- `internal/github` imports `internal/workflow` types only. Keep `workflow`, `gatekeeper`, `journal`, `registry`, and `cli` free of github imports; only `github` knows about GitHub payloads.
- No filesystem or network access in `internal/github` (pure functions on `[]byte`/`string`/structs, suitable for later cursor-based polling).
- Use the canonical terms from `CONTEXT.md` (Work Item, Triage, Spec, Human Gate, Acceptance Gate); never write ticket, task, or card for those meanings.
- Bound every external string: title 1..300 runes, body <= 20000 runes, author login 1..100 runes. Strip ASCII control characters except `\n` and `\t`; trim surrounding space.
- Repository must match `owner/repo` with each part 1..100 runes of `[A-Za-z0-9._-]`; Issue number must be > 0.
- Reconciliation emits at most one Command per call; multi-step jumps are rejected, never synthesized.
- Closing an Issue records contributor intent but emits no state-changing Command; terminal Work Items are never resurrected by intake.
- Do not add generic repository, provider, manager, service, utils, or common packages; new code lives in `internal/github` only.

---

### Task 1: Parse and normalize the GitHub Issue payload

**Files:**
- Create: `internal/github/issue.go`
- Test: `internal/github/issue_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `IssueState string` (`IssueOpen`, `IssueClosed`), `IntakeIssue struct`, `ParseIssue(data []byte) (IntakeIssue, error)`, sentinels `ErrIssue`.

- [ ] **Step 1: Write the failing Issue test**

```go
package github

import (
	"strings"
	"testing"
)

const validIssue = `{"repository": "owner/repo", "number": 123, "title": "Fix login", "body": "Steps to reproduce", "state": "open", "author": "octocat"}`

func TestParseValidIssue(t *testing.T) {
	t.Parallel()

	issue, err := ParseIssue([]byte(validIssue))
	if err != nil {
		t.Fatal(err)
	}
	if string(issue.ID) != "owner/repo#123" {
		t.Fatalf("unexpected id: %q", issue.ID)
	}
	if issue.Title != "Fix login" || issue.State != IssueOpen || issue.Number != 123 {
		t.Fatalf("unexpected issue: %#v", issue)
	}
}

func TestRejectBadIssues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  string
	}{
		{name: "not json", doc: `{"repository":`},
		{name: "missing repository", doc: `{"number": 1, "title": "x", "state": "open", "author": "a"}`},
		{name: "bad repository", doc: `{"repository": "owner", "number": 1, "title": "x", "state": "open", "author": "a"}`},
		{name: "zero number", doc: `{"repository": "owner/repo", "number": 0, "title": "x", "state": "open", "author": "a"}`},
		{name: "empty title", doc: `{"repository": "owner/repo", "number": 1, "title": "  ", "state": "open", "author": "a"}`},
		{name: "title too long", doc: `{"repository": "owner/repo", "number": 1, "title": "` + strings.Repeat("x", 301) + `", "state": "open", "author": "a"}`},
		{name: "body too long", doc: `{"repository": "owner/repo", "number": 1, "title": "x", "body": "` + strings.Repeat("y", 20001) + `", "state": "open", "author": "a"}`},
		{name: "bad state", doc: `{"repository": "owner/repo", "number": 1, "title": "x", "state": "merged", "author": "a"}`},
		{name: "missing author", doc: `{"repository": "owner/repo", "number": 1, "title": "x", "state": "open"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseIssue([]byte(tc.doc)); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestIssueSanitizesControlCharacters(t *testing.T) {
	t.Parallel()

	issue, err := ParseIssue([]byte(`{"repository": "owner/repo", "number": 7, "title": "ab", "body": "x@y", "state": "open", "author": "octocat"}`))
	if err != nil {
		t.Fatal(err)
	}
	if issue.Title != "ab" || issue.Body != "xy" {
		t.Fatalf("controls not stripped: %#v", issue)
	}
}
```

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/github -run 'TestParseValidIssue|TestRejectBadIssues|TestIssueSanitizesControlCharacters' -count=1`

Expected: FAIL because `ParseIssue` and the sentinels are undefined.

- [ ] **Step 3: Implement Issue parsing with bounds and sanitization**

```go
// internal/github/issue.go
package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

var ErrIssue = errors.New("invalid github issue")

type IssueState string

const (
	IssueOpen   IssueState = "open"
	IssueClosed IssueState = "closed"
)

type IntakeIssue struct {
	ID         workflow.WorkItemID
	Repository string
	Number     int
	Title      string
	Body       string
	State      IssueState
	Author     string
}

func ParseIssue(data []byte) (IntakeIssue, error) {
	var raw struct {
		Repository string `json:"repository"`
		Number     int    `json:"number"`
		Title      string `json:"title"`
		Body       string `json:"body"`
		State      string `json:"state"`
		Author     string `json:"author"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return IntakeIssue{}, err
	}
	if !validRepository(raw.Repository) {
		return IntakeIssue{}, fmt.Errorf("%w: repository %q", ErrIssue, raw.Repository)
	}
	if raw.Number <= 0 {
		return IntakeIssue{}, fmt.Errorf("%w: number %d", ErrIssue, raw.Number)
	}
	title := sanitize(raw.Title)
	if utf8.RuneCountInString(title) == 0 || utf8.RuneCountInString(title) > 300 {
		return IntakeIssue{}, fmt.Errorf("%w: title length %d", ErrIssue, utf8.RuneCountInString(title))
	}
	body := sanitize(raw.Body)
	if utf8.RuneCountInString(body) > 20000 {
		return IntakeIssue{}, fmt.Errorf("%w: body too long", ErrIssue)
	}
	var state IssueState
	switch raw.State {
	case string(IssueOpen):
		state = IssueOpen
	case string(IssueClosed):
		state = IssueClosed
	default:
		return IntakeIssue{}, fmt.Errorf("%w: state %q", ErrIssue, raw.State)
	}
	author := sanitize(raw.Author)
	if utf8.RuneCountInString(author) == 0 || utf8.RuneCountInString(author) > 100 {
		return IntakeIssue{}, fmt.Errorf("%w: author", ErrIssue)
	}
	return IntakeIssue{
		ID:         workflow.WorkItemID(fmt.Sprintf("%s#%d", raw.Repository, raw.Number)),
		Repository: raw.Repository,
		Number:     raw.Number,
		Title:      title,
		Body:       body,
		State:      state,
		Author:     author,
	}, nil
}

func validRepository(repo string) bool {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 100 {
			return false
		}
		for _, r := range part {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
```

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/github -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/github/issue.go internal/github/issue_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/github/issue.go internal/github/issue_test.go
git commit -m "feat(github): parse and normalize issue payloads"
```

### Task 2: Parse the GitHub Project status field

**Files:**
- Create: `internal/github/project.go`
- Test: `internal/github/project_test.go`

**Interfaces:**
- Consumes: `workflow.State` values from the Workflow Module.
- Produces: `ParseProjectStatus(s string) (workflow.State, error)`, sentinel `ErrProjectStatus`.

- [ ] **Step 1: Write the failing status test**

```go
package github

import (
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestParseProjectStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   string
		want workflow.State
	}{
		{in: "", want: ""},
		{in: "Inbox", want: workflow.StateInbox},
		{in: "triage", want: workflow.StateTriage},
		{in: "Changes Requested", want: workflow.StateChangesRequested},
		{in: "ready_to_deploy", want: workflow.StateReadyToDeploy},
		{in: "Client QA", want: workflow.StateClientQA},
		{in: "rolling-out", want: workflow.StateRollingOut},
		{in: "DONE", want: workflow.StateDone},
	} {
		got, err := ParseProjectStatus(tc.in)
		if err != nil {
			t.Fatalf("input %q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("input %q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRejectBadProjectStatus(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"Backlog", "In Progress", "Shipped it", "   "} {
		if _, err := ParseProjectStatus(in); err == nil {
			t.Fatalf("expected rejection for %q", in)
		}
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/github -run 'TestParseProjectStatus|TestRejectBadProjectStatus' -count=1`

Expected: FAIL because `ParseProjectStatus` is undefined.

- [ ] **Step 3: Implement status parsing with display-name normalization**

```go
// internal/github/project.go
package github

import (
	"errors"
	"fmt"
	"strings"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

var ErrProjectStatus = errors.New("unknown project status")

func ParseProjectStatus(s string) (workflow.State, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", nil
	}
	flat := strings.ToLower(strings.NewReplacer(" ", "_", "-", "_").Replace(trimmed))
	collapsed := strings.Builder{}
	prev := false
	for _, r := range flat {
		if r == '_' {
			if !prev {
				collapsed.WriteRune(r)
			}
			prev = true
			continue
		}
		prev = false
		collapsed.WriteRune(r)
	}
	state := workflow.State(collapsed.String())
	if !state.Valid() {
		return "", fmt.Errorf("%w: %q", ErrProjectStatus, s)
	}
	return state, nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/github -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/github/project.go internal/github/project_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/github/project.go internal/github/project_test.go
git commit -m "feat(github): parse project status field"
```

### Task 3: Reconcile open Issues and Project status into Commands

**Files:**
- Create: `internal/github/reconcile.go`
- Test: `internal/github/reconcile_test.go`

**Interfaces:**
- Consumes: `IntakeIssue` from Task 1, `workflow.State` from Task 2, `workflow.WorkItem`/`Workflow.Allowed` transitions.
- Produces: `ReconcileResult struct`, `Reconcile(current workflow.WorkItem, issue IntakeIssue, projectStatus workflow.State) (ReconcileResult, error)`, sentinels `ErrIntakeJump`, `ErrIntakeTerminal`.

- [ ] **Step 1: Write the failing reconcile test**

```go
package github

import (
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func openIssue() IntakeIssue {
	issue, err := ParseIssue([]byte(`{"repository": "owner/repo", "number": 123, "title": "Fix login", "state": "open", "author": "octocat"}`))
	if err != nil {
		panic(err)
	}
	return issue
}

func itemIn(state workflow.State) workflow.WorkItem {
	return workflow.WorkItem{ID: "owner/repo#123", State: state, Version: 3}
}

func TestReconcileSubmitsNewWork(t *testing.T) {
	t.Parallel()

	res, err := Reconcile(workflow.WorkItem{}, openIssue(), workflow.StateInbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commands) != 1 || res.Commands[0].Type != workflow.CommandSubmitWork {
		t.Fatalf("unexpected commands: %#v", res.Commands)
	}
	if res.Commands[0].AggregateID != "owner/repo#123" || res.Commands[0].ExpectedVersion != 0 {
		t.Fatalf("unexpected submit: %#v", res.Commands[0])
	}
}

func TestReconcileAdvancesOneStep(t *testing.T) {
	t.Parallel()

	res, err := Reconcile(itemIn(workflow.StateInbox), openIssue(), workflow.StateTriage)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commands) != 1 || res.Commands[0].Type != workflow.CommandBeginTriage {
		t.Fatalf("unexpected commands: %#v", res.Commands)
	}
	if res.Commands[0].ExpectedVersion != 3 || res.IntentRecorded {
		t.Fatalf("unexpected reconcile: %#v", res.Commands[0])
	}
}

func TestReconcileNoOpinionIsNoop(t *testing.T) {
	t.Parallel()

	for _, status := range []workflow.State{"", workflow.StateImplementing} {
		res, err := Reconcile(itemIn(workflow.StateImplementing), openIssue(), status)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Commands) != 0 || res.IntentRecorded {
			t.Fatalf("expected noop, got %#v", res)
		}
	}
}

func TestReconcileRejectsJumps(t *testing.T) {
	t.Parallel()

	if _, err := Reconcile(itemIn(workflow.StateInbox), openIssue(), workflow.StateDone); err == nil {
		t.Fatal("expected jump rejection, got none")
	}
}

func TestReconciledCommandPassesWorkflow(t *testing.T) {
	t.Parallel()

	current := itemIn(workflow.StateInbox)
	res, err := Reconcile(current, openIssue(), workflow.StateTriage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (workflow.Workflow{}).Handle(time.Now(), current, res.Commands[0]); err != nil {
		t.Fatalf("reconciled command rejected by workflow: %v", err)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/github -run 'TestReconcile' -count=1`

Expected: FAIL because `Reconcile` is undefined.

- [ ] **Step 3: Implement single-step reconciliation for open Issues**

```go
// internal/github/reconcile.go
package github

import (
	"errors"
	"fmt"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

var ErrIntakeJump = errors.New("project status jump has no single command")

var ErrIntakeTerminal = errors.New("terminal work item cannot advance by intake")

type ReconcileResult struct {
	Commands       []workflow.Command
	IntentRecorded bool
}

const intakeActor = workflow.ActorID("actor/automation")

var stepCommand = map[[2]workflow.State]workflow.CommandType{
	{workflow.StateInbox, workflow.StateTriage}:            workflow.CommandBeginTriage,
	{workflow.StateInbox, workflow.StateBlocked}:           workflow.CommandBlock,
	{workflow.StateInbox, workflow.StateCancelled}:         workflow.CommandCancel,
	{workflow.StateTriage, workflow.StateReady}:            workflow.CommandAuthorizeWork,
	{workflow.StateTriage, workflow.StateCancelled}:        workflow.CommandRejectWork,
	{workflow.StateTriage, workflow.StateBlocked}:          workflow.CommandBlock,
	{workflow.StateReady, workflow.StateSpecifying}:        workflow.CommandBeginSpec,
	{workflow.StateReady, workflow.StateImplementing}:      workflow.CommandBeginImplementation,
	{workflow.StateReady, workflow.StateBlocked}:           workflow.CommandBlock,
	{workflow.StateReady, workflow.StateCancelled}:         workflow.CommandCancel,
	{workflow.StateSpecifying, workflow.StateReady}:        workflow.CommandApproveSpec,
	{workflow.StateSpecifying, workflow.StateBlocked}:      workflow.CommandBlock,
	{workflow.StateSpecifying, workflow.StateCancelled}:    workflow.CommandCancel,
	{workflow.StateImplementing, workflow.StateReviewing}:  workflow.CommandSubmitReview,
	{workflow.StateImplementing, workflow.StateBlocked}:    workflow.CommandBlock,
	{workflow.StateImplementing, workflow.StateCancelled}:  workflow.CommandCancel,
	{workflow.StateReviewing, workflow.StateChangesRequested}: workflow.CommandRequestChanges,
	{workflow.StateReviewing, workflow.StateReadyToDeploy}:   workflow.CommandApprovePR,
	{workflow.StateReviewing, workflow.StateBlocked}:         workflow.CommandBlock,
	{workflow.StateReviewing, workflow.StateCancelled}:       workflow.CommandCancel,
	{workflow.StateChangesRequested, workflow.StateImplementing}: workflow.CommandBeginImplementation,
	{workflow.StateChangesRequested, workflow.StateCancelled}:     workflow.CommandCancel,
	{workflow.StateReadyToDeploy, workflow.StateDeploying}:  workflow.CommandBeginDeploy,
	{workflow.StateReadyToDeploy, workflow.StateBlocked}:    workflow.CommandBlock,
	{workflow.StateReadyToDeploy, workflow.StateCancelled}:  workflow.CommandCancel,
	{workflow.StateDeploying, workflow.StateClientQA}:       workflow.CommandMarkDeploymentHealthy,
	{workflow.StateDeploying, workflow.StateFailed}:         workflow.CommandFailDeployment,
	{workflow.StateClientQA, workflow.StateRollingOut}:      workflow.CommandAcceptFeature,
	{workflow.StateClientQA, workflow.StateChangesRequested}: workflow.CommandRequestChanges,
	{workflow.StateClientQA, workflow.StateBlocked}:         workflow.CommandBlock,
	{workflow.StateRollingOut, workflow.StateDone}:          workflow.CommandCompleteRollout,
	{workflow.StateRollingOut, workflow.StateBlocked}:       workflow.CommandBlock,
	{workflow.StateFailed, workflow.StateReadyToDeploy}:     workflow.CommandRetryDeployment,
	{workflow.StateFailed, workflow.StateCancelled}:         workflow.CommandCancel,
}

func Reconcile(current workflow.WorkItem, issue IntakeIssue, projectStatus workflow.State) (ReconcileResult, error) {
	if issue.State == IssueClosed {
		return reconcileClosed(current)
	}
	if current == (workflow.WorkItem{}) {
		return ReconcileResult{Commands: []workflow.Command{{
			ID:              intakeCommandID(issue, "", workflow.StateInbox),
			AggregateID:     issue.ID,
			ExpectedVersion: 0,
			ActorID:         intakeActor,
			Type:            workflow.CommandSubmitWork,
			Reason:          issue.Title,
		}}}, nil
	}
	if current.State == workflow.StateDone || current.State == workflow.StateCancelled {
		return ReconcileResult{}, fmt.Errorf("%w: %q", ErrIntakeTerminal, current.State)
	}
	if current.State == workflow.StateBlocked {
		return ReconcileResult{}, fmt.Errorf("%w: resolve blocked explicitly", ErrIntakeJump)
	}
	if projectStatus == "" || projectStatus == current.State {
		return ReconcileResult{}, nil
	}
	cmdType, ok := stepCommand[[2]workflow.State{current.State, projectStatus}]
	if !ok {
		return ReconcileResult{}, fmt.Errorf("%w: %q to %q", ErrIntakeJump, current.State, projectStatus)
	}
	reason := fmt.Sprintf("github intake: %s -> %s", current.State, projectStatus)
	if needsReason(cmdType) {
		reason = issue.Title
	}
	return ReconcileResult{Commands: []workflow.Command{{
		ID:              intakeCommandID(issue, current.State, projectStatus),
		AggregateID:     current.ID,
		ExpectedVersion: current.Version,
		ActorID:         intakeActor,
		Type:            cmdType,
		Reason:          reason,
	}}}, nil
}

func reconcileClosed(current workflow.WorkItem) (ReconcileResult, error) {
	if current == (workflow.WorkItem{}) {
		return ReconcileResult{}, nil
	}
	if current.State == workflow.StateDone || current.State == workflow.StateCancelled {
		return ReconcileResult{}, nil
	}
	return ReconcileResult{IntentRecorded: true}, nil
}

func needsReason(cmdType workflow.CommandType) bool {
	switch cmdType {
	case workflow.CommandBlock, workflow.CommandCancel, workflow.CommandRejectWork, workflow.CommandFailDeployment:
		return true
	default:
		return false
	}
}

func intakeCommandID(issue IntakeIssue, from, to workflow.State) workflow.CommandID {
	if from == "" {
		return workflow.CommandID(fmt.Sprintf("intake-%d-submit", issue.Number))
	}
	return workflow.CommandID(fmt.Sprintf("intake-%d-%s-%s", issue.Number, from, to))
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/github -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/github/reconcile.go internal/github/reconcile_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/github/reconcile.go internal/github/reconcile_test.go
git commit -m "feat(github): reconcile open issues into commands"
```

### Task 4: Prove closed-Issue intent, blocked safety, and idempotent command identity

**Files:**
- Create: `internal/github/intake_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete `Reconcile` from Task 3.
- Produces: executable evidence that closing an Issue never mutates State, blocked items never jump by intake, and reconciled Commands carry stable idempotency keys.

- [ ] **Step 1: Add the intake safety test**

```go
package github

import (
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func closedIssue() IntakeIssue {
	issue, err := ParseIssue([]byte(`{"repository": "owner/repo", "number": 123, "title": "Fix login", "state": "closed", "author": "octocat"}`))
	if err != nil {
		panic(err)
	}
	return issue
}

func TestClosedIssueRecordsIntentWithoutCommands(t *testing.T) {
	t.Parallel()

	for _, state := range []workflow.State{workflow.StateClientQA, workflow.StateReadyToDeploy, workflow.StateImplementing} {
		res, err := Reconcile(itemIn(state), closedIssue(), workflow.StateDone)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Commands) != 0 || !res.IntentRecorded {
			t.Fatalf("state %q: expected intent without commands, got %#v", state, res)
		}
	}
}

func TestClosedIssueWithoutWorkIsNoop(t *testing.T) {
	t.Parallel()

	res, err := Reconcile(workflow.WorkItem{}, closedIssue(), workflow.StateDone)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commands) != 0 || res.IntentRecorded {
		t.Fatalf("expected noop, got %#v", res)
	}
}

func TestBlockedNeverJumpsByIntake(t *testing.T) {
	t.Parallel()

	if _, err := Reconcile(itemIn(workflow.StateBlocked), openIssue(), workflow.StateReady); err == nil {
		t.Fatal("expected blocked jump rejection, got none")
	}
}

func TestTerminalNeverResurrectsByIntake(t *testing.T) {
	t.Parallel()

	for _, state := range []workflow.State{workflow.StateDone, workflow.StateCancelled} {
		if _, err := Reconcile(itemIn(state), openIssue(), workflow.StateInbox); err == nil {
			t.Fatalf("state %q: expected terminal rejection", state)
		}
		res, err := Reconcile(itemIn(state), closedIssue(), workflow.StateDone)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Commands) != 0 {
			t.Fatalf("terminal must emit no commands, got %#v", res)
		}
	}
}

func TestIntakeCommandIDsAreStable(t *testing.T) {
	t.Parallel()

	first, err := Reconcile(itemIn(workflow.StateInbox), openIssue(), workflow.StateTriage)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Reconcile(itemIn(workflow.StateInbox), openIssue(), workflow.StateTriage)
	if err != nil {
		t.Fatal(err)
	}
	if first.Commands[0].ID != second.Commands[0].ID {
		t.Fatalf("unstable command id: %q vs %q", first.Commands[0].ID, second.Commands[0].ID)
	}
}
```

- [ ] **Step 2: Run the intake safety test**

Run: `go test ./internal/github -run 'TestClosed|TestBlocked|TestTerminal|TestIntake' -count=1`

Expected: PASS (reconcile.go already implements these rules; the test locks them).

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean build of `cmd/harness`.

- [ ] **Step 4: Mark Increment 4 verified and commit**

In `docs/implementation-plan.md`, add Increment 4 to the plan index and a verification checklist below Increment 3. Do not mark it complete until the commands above pass on master.

```bash
git add internal/github/intake_test.go docs/implementation-plan.md
git commit -m "test(github): verify intake safety and idempotency"
```
