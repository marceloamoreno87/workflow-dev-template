# Operational Journal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist authenticated Commands and immutable Events in SQLite so Work Item State rebuilds after restart without changing workflow and gatekeeper interfaces.

**Architecture:** `journal` owns SQLite persistence, optimistic concurrency, idempotency by CommandID, and leases. `workflow` owns transitions and folding, `gatekeeper` owns authorization; both stay pure with no SQLite, driver, or network imports. `journal` imports `workflow` only, calls `Workflow.Handle`, then persists the returned Events atomically. Projections rebuild exclusively via `workflow.Fold`.

**Tech Stack:** Go 1.27.1 standard library plus `modernc.org/sqlite` (pure-Go driver, single binary, no CGO), `database/sql`, table-driven tests

**Spec:** `docs/architecture.md`, `docs/workflow.md`, `docs/contracts.md`, `docs/adr/0005-keep-orchestration-state-reconstructible.md`, `docs/adr/0018-drive-workflow-with-commands-and-events.md`, `docs/adr/0022-enforce-idempotent-optimistic-commands.md`

## Global Constraints

- Use Go 1.27.1.
- Keep `workflow` and `gatekeeper` free of I/O, SQL, and third-party imports; only `journal` may import the SQLite driver.
- Use the canonical terms from `CONTEXT.md` (Work Item, Command, Event, Execution, Project Worktree, Human Gate, Quality Profile, Orchestration State); never write task, ticket, sandbox, branch, agent, bot, log, or notification for those meanings.
- A Command is idempotent by `CommandID`; a duplicate `CommandID` returns the prior Events without appending.
- Every accepted Command emits at least one Event; every rejected Command emits none.
- State changes occur only by folding Events with `workflow.Fold`.
- SQLite runs in WAL mode with one writer (`SetMaxOpenConns(1)`); projections are rebuildable and disposable.
- Credentials, tokens, and secrets never enter the journal, prompts, or logs.
- Do not add generic repository, provider, manager, service, utils, or common packages; the new code lives in `internal/journal` only.

---

### Task 1: Bootstrap the journal Module with migrations and WAL

**Files:**
- Create: `internal/journal/journal.go`
- Test: `internal/journal/journal_test.go`

**Interfaces:**
- Consumes: nothing (only `database/sql`, driver, `workflow` types for later tasks).
- Produces: `Store struct`, `Open(path string) (*Store, error)`, `(*Store).Close() error`, schema tables `schema_migrations`, `commands`, `events`, `leases`.

- [ ] **Step 1: Write the failing open-and-migrate test**

```go
package journal

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "journal.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, table := range []string{"schema_migrations", "commands", "events", "leases"} {
		var name string
		err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %q missing: %v", table, err)
		}
	}

	var mode string
	if err := s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}
```

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/journal -run TestOpenCreatesSchema -count=1`

Expected: FAIL because `Open`, `Store`, and `s.db` are undefined.

- [ ] **Step 3: Add the SQLite dependency and minimal Store**

Run: `go get modernc.org/sqlite@latest`

```go
// internal/journal/journal.go
package journal

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS commands (
	command_id TEXT PRIMARY KEY,
	aggregate_id TEXT NOT NULL,
	expected_version INTEGER NOT NULL,
	actor_id TEXT NOT NULL,
	type TEXT NOT NULL,
	reason TEXT NOT NULL,
	received_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
	aggregate_id TEXT NOT NULL,
	version INTEGER NOT NULL,
	command_id TEXT NOT NULL,
	actor_id TEXT NOT NULL,
	type TEXT NOT NULL,
	from_state TEXT NOT NULL,
	to_state TEXT NOT NULL,
	reason TEXT NOT NULL,
	at TEXT NOT NULL,
	PRIMARY KEY (aggregate_id, version)
);
CREATE INDEX IF NOT EXISTS idx_events_command ON events(command_id);
CREATE TABLE IF NOT EXISTS leases (
	resource TEXT PRIMARY KEY,
	holder TEXT NOT NULL,
	expires_at TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ','now'));
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
```

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/journal -run TestOpenCreatesSchema -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/journal/journal.go internal/journal/journal_test.go && go test ./internal/journal -count=1`

Expected: PASS.

```bash
git add go.mod go.sum internal/journal/journal.go internal/journal/journal_test.go
git commit -m "feat(journal): open sqlite store with wal schema"
```

### Task 2: Record Commands idempotently

**Files:**
- Modify: `internal/journal/journal.go`
- Modify: `internal/journal/journal_test.go`

**Interfaces:**
- Consumes: `workflow.Command`, `Store` from Task 1.
- Produces: `(*Store).Apply` idempotency path, `ErrConflict` sentinel, time encoding helper.

- [ ] **Step 1: Write the failing idempotency test**

```go
package journal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestApplyIsIdempotentByCommandID(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	cmd := workflow.Command{ID: "c1", AggregateID: "repo#1", ExpectedVersion: 0, ActorID: "operator", Type: workflow.CommandSubmitWork}
	first, err := s.Apply(time.Unix(1, 0).UTC(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Version != 1 {
		t.Fatalf("unexpected first events: %#v", first)
	}

	second, err := s.Apply(time.Unix(2, 0).UTC(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Version != 1 || second[0].CommandID != "c1" {
		t.Fatalf("unexpected replayed events: %#v", second)
	}

	count := 0
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE aggregate_id='repo#1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("event count = %d, want 1", count)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/journal -run TestApplyIsIdempotentByCommandID -count=1`

Expected: FAIL because `Apply` and `ErrConflict` are undefined.

- [ ] **Step 3: Implement Apply with duplicate short-circuit and optimistic check**

```go
package journal

import (
	"database/sql"
	"errors"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

var ErrConflict = errors.New("stale expected version")

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

func (s *Store) Apply(now time.Time, cmd workflow.Command) ([]workflow.Event, error) {
	now = now.UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var commandID, aggregateID, actorID, commandType, reason, receivedAt string
	var expectedVersion uint64
	err = tx.QueryRow(`SELECT command_id, aggregate_id, expected_version, actor_id, type, reason, received_at FROM commands WHERE command_id=?`, string(cmd.ID)).Scan(&commandID, &aggregateID, &expectedVersion, &actorID, &commandType, &reason, &receivedAt)
	if err == nil {
		rows, err := tx.Query(`SELECT aggregate_id, version, command_id, actor_id, type, from_state, to_state, reason, at FROM events WHERE command_id=? ORDER BY version`, string(cmd.ID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		events, err := scanEvents(rows)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return events, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	current, err := loadWorkItemTx(tx, cmd.AggregateID)
	if err != nil {
		return nil, err
	}
	if cmd.ExpectedVersion != current.Version {
		return nil, ErrConflict
	}
	events, err := (workflow.Workflow{}).Handle(now, current, cmd)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO commands(command_id, aggregate_id, expected_version, actor_id, type, reason, received_at) VALUES(?,?,?,?,?,?,?)`, string(cmd.ID), string(cmd.AggregateID), uint64(cmd.ExpectedVersion), string(cmd.ActorID), string(cmd.Type), cmd.Reason, formatTime(now)); err != nil {
		return nil, err
	}
	for _, e := range events {
		if _, err := tx.Exec(`INSERT INTO events(aggregate_id, version, command_id, actor_id, type, from_state, to_state, reason, at) VALUES(?,?,?,?,?,?,?,?,?)`, string(e.AggregateID), uint64(e.Version), string(e.CommandID), string(e.ActorID), string(e.Type), string(e.From), string(e.To), e.Reason, formatTime(e.At)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return events, nil
}

func scanEvents(rows *sql.Rows) ([]workflow.Event, error) {
	var out []workflow.Event
	for rows.Next() {
		var aggregateID, commandID, actorID, eventType, fromState, toState, reason, at string
		var version uint64
		if err := rows.Scan(&aggregateID, &version, &commandID, &actorID, &eventType, &fromState, &toState, &reason, &at); err != nil {
			return nil, err
		}
		parsed, err := parseTime(at)
		if err != nil {
			return nil, err
		}
		out = append(out, workflow.Event{
			AggregateID: workflow.WorkItemID(aggregateID),
			Version:     workflow.Version(version),
			CommandID:   workflow.CommandID(commandID),
			ActorID:     workflow.ActorID(actorID),
			Type:        workflow.EventType(eventType),
			From:        workflow.State(fromState),
			To:          workflow.State(toState),
			Reason:      reason,
			At:          parsed,
		})
	}
	return out, rows.Err()
}

func loadWorkItemTx(tx *sql.Tx, id workflow.WorkItemID) (workflow.WorkItem, error) {
	rows, err := tx.Query(`SELECT aggregate_id, version, command_id, actor_id, type, from_state, to_state, reason, at FROM events WHERE aggregate_id=? ORDER BY version`, string(id))
	if err != nil {
		return workflow.WorkItem{}, err
	}
	defer rows.Close()
	events, err := scanEvents(rows)
	if err != nil {
		return workflow.WorkItem{}, err
	}
	if len(events) == 0 {
		return workflow.WorkItem{}, nil
	}
	return workflow.Fold(events)
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/journal -count=1`

Expected: PASS (schema test plus idempotency test).

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/journal/journal.go internal/journal/journal_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/journal/journal.go internal/journal/journal_test.go
git commit -m "feat(journal): apply commands idempotently"
```

### Task 3: Enforce optimistic concurrency and rebuild State

**Files:**
- Modify: `internal/journal/journal.go`
- Modify: `internal/journal/journal_test.go`

**Interfaces:**
- Consumes: `workflow.Fold`, `workflow.Workflow.Allowed`, `Store.Apply` from Task 2.
- Produces: `(*Store).Load(id workflow.WorkItemID) (workflow.WorkItem, error)`, `ErrNotFound` sentinel.

- [ ] **Step 1: Write the failing stale-version and load tests**

```go
package journal

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestApplyRejectsStaleVersionWithoutNewEvents(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.Apply(time.Unix(1, 0).UTC(), workflow.Command{ID: "c1", AggregateID: "repo#1", ExpectedVersion: 0, ActorID: "operator", Type: workflow.CommandSubmitWork}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(time.Unix(2, 0).UTC(), workflow.Command{ID: "c2", AggregateID: "repo#1", ExpectedVersion: 1, ActorID: "operator", Type: workflow.CommandBeginTriage}); err != nil {
		t.Fatal(err)
	}

	_, err = s.Apply(time.Unix(3, 0).UTC(), workflow.Command{ID: "c3", AggregateID: "repo#1", ExpectedVersion: 1, ActorID: "operator", Type: workflow.CommandAuthorizeWork})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Apply() error = %v, want ErrConflict", err)
	}

	got, err := s.Load("repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != workflow.StateTriage || got.Version != 2 {
		t.Fatalf("unexpected work item: %#v", got)
	}
}

func TestLoadRebuildsStateAndReportsMissing(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.Load("missing#1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}

	if _, err := s.Apply(time.Unix(1, 0).UTC(), workflow.Command{ID: "c1", AggregateID: "repo#9", ExpectedVersion: 0, ActorID: "operator", Type: workflow.CommandSubmitWork}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("repo#9")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "repo#9" || got.State != workflow.StateInbox || got.Version != 1 {
		t.Fatalf("unexpected work item: %#v", got)
	}
}
```

- [ ] **Step 2: Run the tests and verify Load is undefined**

Run: `go test ./internal/journal -run 'TestApplyRejectsStale|TestLoadRebuilds' -count=1`

Expected: FAIL because `Load` and `ErrNotFound` are undefined.

- [ ] **Step 3: Implement Load via Fold**

```go
var ErrNotFound = errors.New("work item not found")

func (s *Store) Load(id workflow.WorkItemID) (workflow.WorkItem, error) {
	rows, err := s.db.Query(`SELECT aggregate_id, version, command_id, actor_id, type, from_state, to_state, reason, at FROM events WHERE aggregate_id=? ORDER BY version`, string(id))
	if err != nil {
		return workflow.WorkItem{}, err
	}
	defer rows.Close()
	events, err := scanEvents(rows)
	if err != nil {
		return workflow.WorkItem{}, err
	}
	if len(events) == 0 {
		return workflow.WorkItem{}, ErrNotFound
	}
	return workflow.Fold(events)
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/journal -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/journal/journal.go internal/journal/journal_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/journal/journal.go internal/journal/journal_test.go
git commit -m "feat(journal): enforce optimistic concurrency and rebuild state"
```

### Task 4: Coordinate Executions with leases

**Files:**
- Modify: `internal/journal/journal.go`
- Modify: `internal/journal/journal_test.go`

**Interfaces:**
- Consumes: `Store` from Tasks 1-3.
- Produces: `AcquireLease(resource, holder string, ttl time.Duration, now time.Time) (bool, error)`, `RenewLease`, `ReleaseLease`.

- [ ] **Step 1: Write the failing lease test**

```go
package journal

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLeaseAcquireRenewRelease(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Unix(100, 0).UTC()
	ok, err := s.AcquireLease("repo#1", "exec-a", time.Minute, now)
	if err != nil || !ok {
		t.Fatalf("AcquireLease() = %v, %v; want true, nil", ok, err)
	}
	ok, err = s.AcquireLease("repo#1", "exec-b", time.Minute, now)
	if err != nil || ok {
		t.Fatalf("second AcquireLease() = %v, %v; want false, nil", ok, err)
	}
	ok, err = s.RenewLease("repo#1", "exec-b", time.Minute, now)
	if err != nil || ok {
		t.Fatalf("foreign RenewLease() = %v, %v; want false, nil", ok, err)
	}
	ok, err = s.RenewLease("repo#1", "exec-a", time.Minute, now)
	if err != nil || !ok {
		t.Fatalf("RenewLease() = %v, %v; want true, nil", ok, err)
	}
	ok, err = s.ReleaseLease("repo#1", "exec-b")
	if err != nil || ok {
		t.Fatalf("foreign ReleaseLease() = %v, %v; want false, nil", ok, err)
	}
	ok, err = s.ReleaseLease("repo#1", "exec-a")
	if err != nil || !ok {
		t.Fatalf("ReleaseLease() = %v, %v; want true, nil", ok, err)
	}
	ok, err = s.AcquireLease("repo#1", "exec-b", time.Minute, now)
	if err != nil || !ok {
		t.Fatalf("re-acquire = %v, %v; want true, nil", ok, err)
	}
}

func TestExpiredLeaseIsReacquirable(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Unix(200, 0).UTC()
	if _, err := s.AcquireLease("repo#2", "exec-a", time.Minute, now); err != nil {
		t.Fatal(err)
	}
	ok, err := s.AcquireLease("repo#2", "exec-b", time.Minute, now.Add(2*time.Minute))
	if err != nil || !ok {
		t.Fatalf("expired AcquireLease() = %v, %v; want true, nil", ok, err)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/journal -run TestLease -count=1`

Expected: FAIL because lease methods are undefined.

- [ ] **Step 3: Implement leases with expiry comparison**

```go
func (s *Store) AcquireLease(resource, holder string, ttl time.Duration, now time.Time) (bool, error) {
	now = now.UTC()
	expires := formatTime(now.Add(ttl))
	res, err := s.db.Exec(`INSERT INTO leases(resource, holder, expires_at) VALUES(?,?,?) ON CONFLICT(resource) DO UPDATE SET holder=excluded.holder, expires_at=excluded.expires_at WHERE leases.expires_at <= ?`, resource, holder, expires, formatTime(now))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (s *Store) RenewLease(resource, holder string, ttl time.Duration, now time.Time) (bool, error) {
	now = now.UTC()
	res, err := s.db.Exec(`UPDATE leases SET expires_at=? WHERE resource=? AND holder=? AND expires_at > ?`, formatTime(now.Add(ttl)), resource, holder, formatTime(now))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (s *Store) ReleaseLease(resource, holder string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM leases WHERE resource=? AND holder=?`, resource, holder)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/journal -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/journal/journal.go internal/journal/journal_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/journal/journal.go internal/journal/journal_test.go
git commit -m "feat(journal): coordinate executions with leases"
```

### Task 5: Prove restart recovery and journal safety

**Files:**
- Create: `internal/journal/recovery_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete `Store.Apply`, `Store.Load`, `Gatekeeper.Policy.Decide` from Tasks 1-4 plus Increment 1.
- Produces: executable evidence that persisted Events rebuild identical State after reopen and invalid Commands never mutate State.

- [ ] **Step 1: Add the recovery scenario test**

```go
package journal_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/gatekeeper"
	"github.com/marceloamoreno87/workflow-dev-template/internal/journal"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestRestartRecoversHappyPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "journal.db")
	s, err := journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	p := gatekeeper.Policy{}
	profile := gatekeeper.ProfileStandard
	sequence := []workflow.CommandType{
		workflow.CommandSubmitWork, workflow.CommandBeginTriage, workflow.CommandAuthorizeWork,
		workflow.CommandBeginSpec, workflow.CommandApproveSpec, workflow.CommandBeginImplementation,
		workflow.CommandSubmitReview, workflow.CommandApprovePR, workflow.CommandBeginDeploy,
		workflow.CommandMarkDeploymentHealthy, workflow.CommandAcceptFeature, workflow.CommandCompleteRollout,
	}

	now := time.Unix(1, 0).UTC()
	item := workflow.WorkItem{}
	for i, commandType := range sequence {
		cmd := workflow.Command{ID: workflow.CommandID(fmt.Sprintf("c%d", i+1)), AggregateID: "repo#1", ExpectedVersion: item.Version, ActorID: "operator", Type: commandType}
		state := item.State
		if i == 0 {
			if d := p.Decide(gatekeeper.Context{Actor: gatekeeper.ActorOperator, Profile: profile}, cmd); !d.Allowed {
				t.Fatalf("gatekeeper denied SubmitWork: %#v", d)
			}
		} else if d := p.Decide(gatekeeper.Context{State: state, Actor: gatekeeper.ActorOperator, Profile: profile}, cmd); !d.Allowed {
			t.Fatalf("gatekeeper denied %q: %#v", commandType, d)
		}
		events, err := s.Apply(now, cmd)
		if err != nil {
			t.Fatalf("Apply(%q) error = %v", commandType, err)
		}
		if len(events) == 0 {
			t.Fatalf("Apply(%q) returned no events", commandType)
		}
		loaded, err := s.Load("repo#1")
		if err != nil {
			t.Fatal(err)
		}
		item = loaded
		now = now.Add(time.Second)
	}
	before, err := s.Load("repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if before.State != workflow.StateDone || before.Version != workflow.Version(len(sequence)) {
		t.Fatalf("before restart = %#v", before)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	after, err := reopened.Load("repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("after restart = %#v, want %#v", after, before)
	}

	stale := workflow.Command{ID: "stale", AggregateID: "repo#1", ExpectedVersion: 1, ActorID: "operator", Type: workflow.CommandBeginTriage}
	if _, err := reopened.Apply(now, stale); err == nil {
		t.Fatal("stale Apply expected error")
	}
	unchanged, err := reopened.Load("repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged != after {
		t.Fatalf("stale command mutated state: %#v vs %#v", unchanged, after)
	}
}
```

- [ ] **Step 2: Run the scenario test**

Run: `go test ./internal/journal -run TestRestartRecoversHappyPath -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

- [ ] **Step 4: Mark Increment 2 verified and commit**

In `docs/implementation-plan.md`, add a checklist below Increment 2 with the commands and evidence required to mark it complete. Do not mark it complete until the commands above pass in the implementation Project Worktree.

```bash
git add internal/journal/recovery_test.go docs/implementation-plan.md
git commit -m "test(journal): verify restart recovery"
```
