# Daemon Skeleton Implementation Plan (Increment 17)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `harnessd` boots from a validated config file, opens the journal, serves the
loopback dashboard off real projections, ticks intake into the journal through the
Gatekeeper, persists its known-item cursor, and shuts down gracefully with zero work lost.

**Architecture:** `daemon` owns lifecycle only — config loading, journal open/close,
HTTP serving, tick dispatch, and cursor persistence. It owns no policy: submit
eligibility comes from `workflow`+`gatekeeper`, persistence from `journal`, views from
`dashboard`. It may import `internal/journal`, `internal/workflow`, `internal/gatekeeper`,
`internal/dashboard` plus the standard library and `gopkg.in/yaml.v3`. All other modules
stay untouched with no new imports. The GitHub poller, role pipeline, and MCP server
arrive in later increments; the tick consumes an `IntakeSource` interface fed by fakes
in tests and an empty source in production until Increment 20.

**Tech Stack:** Go 1.27.1 standard library plus `gopkg.in/yaml.v3` and the existing
`modernc.org/sqlite` (via `journal`), `net/http` serving, table-driven tests.

**Spec:** `docs/superpowers/plans/2026-09-22-harness-organism.md` (Increment 17),
`docs/architecture.md` (local runtime: `harnessd` as `systemd --user` service, SQLite
WAL one writer, rebuildable projections), `docs/adr/0008-reconcile-github-by-polling.md`.

## Global Constraints

- Use Go 1.27.1. Standard library plus `internal/journal`, `internal/workflow`,
  `internal/gatekeeper`, `internal/dashboard`, and `gopkg.in/yaml.v3` only.
- Use the canonical terms from `CONTEXT.md` (Harness Workspace, Work Item, Execution,
  Command, Event, Gatekeeper, Orchestration State); never write session, job, or ticket.
- Config carries no secrets: the operator token arrives via `tokenFile` (read once at
  load, ≥16 bytes); config parsing is strict (unknown fields rejected).
- `workspace` in config must be absolute; `pollInterval` parses as a Go duration in
  5s..1h; `bind` must be loopback (`127.0.0.1`, `::1`, or `localhost`).
- Every intake item becomes at most one `submit_work` Command (actor `actor/automation`),
  only after a Gatekeeper allow, only when the journal does not know the item; re-polls
  are no-ops, never duplicates.
- The known-item cursor persists to `<workspace>/.harness/daemon-known.json` on every
  successful tick and reloads at boot (bounded file, validated entries); the dashboard
  store is fed from journal loads after every tick and at boot.
- `Run` fails fast on boot errors (bad config, unopenable journal, busy bind) and stays
  up across transient tick errors; cancellation drains the listener and closes the
  journal before returning nil.
- Tests never open live listeners except through `Run` with ephemeral loopback ports,
  and never touch the network.
- Do not add generic repository, provider, manager, service, utils, or common packages;
  new code lives in `internal/daemon` and `cmd/harnessd` only.

---

### Task 1: Load and validate daemon config

**Files:**
- Create: `internal/daemon/config.go`
- Test: `internal/daemon/config_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `Config struct`, `(Config).Validate() error`, `LoadConfig(path string) (Config, error)`, sentinel `ErrDaemon`.

- [ ] **Step 1: Write the failing config test**

```go
package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, workspace, body string) string {
	t.Helper()

	dir := t.TempDir()
	token := filepath.Join(dir, "token")
	if err := os.WriteFile(token, []byte("operator-token-at-least-16"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "daemon.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func validBody(workspace string) string {
	return "schemaVersion: harness.daemon/v1\nworkspace: " + workspace + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\n"
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg, err := LoadConfig(writeConfig(t, root, validBody(root)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspaceRoot != root || cfg.PollInterval != 30*time.Second || cfg.BindAddr != "127.0.0.1:8080" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestRejectBadConfigs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "bad schema", body: "schemaVersion: harness/v9\nworkspace: " + root + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\n"},
		{name: "unknown field", body: validBody(root) + "teleport: true\n"},
		{name: "relative workspace", body: validBody("ws")},
		{name: "bad interval", body: "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 1s\nbind: 127.0.0.1:8080\ntokenFile: token\n"},
		{name: "non-loopback bind", body: "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 30s\nbind: 0.0.0.0:8080\ntokenFile: token\n"},
		{name: "missing token", body: "schemaVersion: harness.daemon/v1\nworkspace: " + root + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: absent\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadConfig(writeConfig(t, root, tc.body)); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestRejectShortToken(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "token")
	if err := os.WriteFile(token, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "daemon.yaml")
	body := "schemaVersion: harness.daemon/v1\nworkspace: " + dir + "\npollInterval: 30s\nbind: 127.0.0.1:8080\ntokenFile: token\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected short-token rejection, got none")
	}
}
```

NOTE: `tokenFile: token` resolves relative to the config file's directory (predictable for operators); document it on `LoadConfig`.

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/daemon -run 'TestLoadConfig|TestRejectBadConfigs|TestRejectShortToken' -count=1`

Expected: FAIL because `Config`, `LoadConfig`, and `ErrDaemon` are undefined.

- [ ] **Step 3: Implement config loading**

```go
// internal/daemon/config.go
package daemon

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var ErrDaemon = errors.New("invalid daemon configuration")

type Config struct {
	WorkspaceRoot string
	PollInterval  time.Duration
	BindAddr      string
	Token         string
}

func (c Config) Validate() error {
	if !filepath.IsAbs(c.WorkspaceRoot) {
		return fmt.Errorf("%w: workspace must be absolute", ErrDaemon)
	}
	if c.PollInterval < 5*time.Second || c.PollInterval > time.Hour {
		return fmt.Errorf("%w: poll interval outside 5s..1h", ErrDaemon)
	}
	host, _, err := net.SplitHostPort(c.BindAddr)
	if err != nil || host == "" {
		return fmt.Errorf("%w: bind %q", ErrDaemon, c.BindAddr)
	}
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("%w: bind %q is not loopback", ErrDaemon, c.BindAddr)
		}
	}
	if len([]byte(c.Token)) < 16 {
		return fmt.Errorf("%w: operator token too short", ErrDaemon)
	}
	return nil
}

func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("%w: unreadable config", ErrDaemon)
	}
	if len(raw) > 64*1024 {
		return Config{}, fmt.Errorf("%w: config too large", ErrDaemon)
	}
	var doc struct {
		SchemaVersion string `yaml:"schemaVersion"`
		Workspace     string `yaml:"workspace"`
		PollInterval  string `yaml:"pollInterval"`
		Bind          string `yaml:"bind"`
		TokenFile     string `yaml:"tokenFile"`
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return Config{}, fmt.Errorf("%w: malformed config", ErrDaemon)
	}
	if doc.SchemaVersion != "harness.daemon/v1" {
		return Config{}, fmt.Errorf("%w: schema %q", ErrDaemon, doc.SchemaVersion)
	}
	interval, err := time.ParseDuration(doc.PollInterval)
	if err != nil {
		return Config{}, fmt.Errorf("%w: poll interval", ErrDaemon)
	}
	tokenPath := doc.TokenFile
	if tokenPath == "" {
		return Config{}, fmt.Errorf("%w: token file required", ErrDaemon)
	}
	if !filepath.IsAbs(tokenPath) {
		tokenPath = filepath.Join(filepath.Dir(path), tokenPath)
	}
	tokenRaw, err := os.ReadFile(tokenPath)
	if err != nil {
		return Config{}, fmt.Errorf("%w: unreadable token file", ErrDaemon)
	}
	cfg := Config{
		WorkspaceRoot: doc.Workspace,
		PollInterval:  interval,
		BindAddr:      doc.Bind,
		Token:         strings.TrimSpace(string(tokenRaw)),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
```

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/daemon -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/daemon/config.go internal/daemon/config_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/daemon/config.go internal/daemon/config_test.go
git commit -m "feat(daemon): load and validate config"
```

### Task 2: Open, tick, persist, feed, close

**Files:**
- Create: `internal/daemon/daemon.go`
- Test: `internal/daemon/daemon_test.go`

**Interfaces:**
- Consumes: `Config` from Task 1; `journal`, `workflow`, `gatekeeper`, `dashboard`.
- Produces: `IntakeItem`, `IntakeSource`, `EmptySource()`, `Daemon`, `Open(cfg, source)`,
  `(Daemon).Close()`, `(Daemon).Tick(now) (bool, error)`, `(Daemon).Dashboard()`.

- [ ] **Step 1: Write the failing daemon test**

```go
package daemon

import (
	"errors"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/journal"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

type fakeSource struct {
	items []IntakeItem
	err   error
}

func (f *fakeSource) Poll(since time.Time, limit int) ([]IntakeItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func testConfig(root string) Config {
	return Config{WorkspaceRoot: root, PollInterval: 5 * time.Second, BindAddr: "127.0.0.1:0", Token: "operator-token-at-least-16"}
}

func TestTickSubmitsUnknownItems(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{
		{ID: "owner/repo#1", Title: "First"},
		{ID: "owner/repo#2", Title: "Second"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	now := time.Now()
	didWork, err := d.Tick(now)
	if err != nil {
		t.Fatal(err)
	}
	if !didWork {
		t.Fatal("expected work, got none")
	}
	item, err := d.Journal().Load("owner/repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if item.State != workflow.StateInbox || item.Version != 1 {
		t.Fatalf("unexpected state: %#v", item)
	}
	again, err := d.Tick(now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("re-poll must be a no-op")
	}
	stable, err := d.Journal().Load("owner/repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if stable.Version != 1 {
		t.Fatalf("duplicate submit: %#v", stable)
	}
}

func TestTickFeedsDashboard(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#9"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Tick(time.Now()); err != nil {
		t.Fatal(err)
	}
	rows := d.Dashboard().Items().List()
	if len(rows) != 1 || rows[0].ID != "owner/repo#9" || rows[0].State != workflow.StateInbox {
		t.Fatalf("dashboard not fed: %#v", rows)
	}
}

func TestRestartRecoversCursor(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Tick(time.Now()); err != nil {
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

	didWork, err := reopened.Tick(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if didWork {
		t.Fatal("reopened daemon must not resubmit known items")
	}
	if rows := reopened.Dashboard().Items().List(); len(rows) != 1 {
		t.Fatalf("dashboard not refed after restart: %#v", rows)
	}
}

func TestTickPropagatesSourceErrors(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{err: errors.New("source down")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Tick(time.Now()); err == nil {
		t.Fatal("expected source error, got none")
	}
}
```

NOTE: `d.Journal()` accessor is needed for the assertions; add it alongside `Dashboard()`.

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/daemon -run 'TestTick|TestRestart' -count=1`

Expected: FAIL because `Daemon`, `Open`, `IntakeSource`, and friends are undefined.

- [ ] **Step 3: Implement the daemon tick**

```go
// internal/daemon/daemon.go
package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/dashboard"
	"github.com/marceloamoreno87/workflow-dev-template/internal/gatekeeper"
	"github.com/marceloamoreno87/workflow-dev-template/internal/journal"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

type IntakeItem struct {
	ID    workflow.WorkItemID
	Title string
}

type IntakeSource interface {
	Poll(since time.Time, limit int) ([]IntakeItem, error)
}

type emptySource struct{}

func EmptySource() IntakeSource { return emptySource{} }

func (emptySource) Poll(time.Time, int) ([]IntakeItem, error) { return nil, nil }

const knownFileName = "daemon-known.json"

const maxKnownItems = 100000

type Daemon struct {
	cfg      Config
	db       *journal.Store
	dash     *dashboard.Server
	source   IntakeSource
	lastPoll time.Time
	known    map[workflow.WorkItemID]bool
}

func harnessDir(root string) string {
	return filepath.Join(root, ".harness")
}

func Open(cfg Config, source IntakeSource) (*Daemon, error) {
	if err := cfg.Validate(); err != nil {
		return Daemon{}, err
	}
	if source == nil {
		return Daemon{}, fmt.Errorf("%w: intake source required", ErrDaemon)
	}
	if err := os.MkdirAll(harnessDir(cfg.WorkspaceRoot), 0o755); err != nil {
		return Daemon{}, err
	}
	db, err := journal.Open(filepath.Join(harnessDir(cfg.WorkspaceRoot), "harness.db"))
	if err != nil {
		return Daemon{}, err
	}
	d := &Daemon{cfg: cfg, db: db, source: source, known: map[workflow.WorkItemID]bool{}}
	if err := d.loadKnown(); err != nil {
		_ = db.Close()
		return Daemon{}, err
	}
	dash, err := dashboard.NewServer(dashboard.Config{BindAddr: cfg.BindAddr, Token: cfg.Token})
	if err != nil {
		_ = db.Close()
		return Daemon{}, err
	}
	d.dash = dash
	d.feedDashboard()
	return d, nil
}

func (d *Daemon) Close() error {
	return d.db.Close()
}

func (d *Daemon) Journal() *journal.Store {
	return d.db
}

func (d *Daemon) Dashboard() *dashboard.Server {
	return d.dash
}

func (d *Daemon) knownPath() string {
	return filepath.Join(harnessDir(d.cfg.WorkspaceRoot), knownFileName)
}

func (d *Daemon) loadKnown() error {
	raw, err := os.ReadFile(d.knownPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(raw) > 4*1024*1024 {
		return fmt.Errorf("%w: known file too large", ErrDaemon)
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return fmt.Errorf("%w: corrupt known file", ErrDaemon)
	}
	if len(ids) > maxKnownItems {
		return fmt.Errorf("%w: too many known items", ErrDaemon)
	}
	for _, id := range ids {
		if id == "" {
			return fmt.Errorf("%w: empty known id", ErrDaemon)
		}
		d.known[workflow.WorkItemID(id)] = true
	}
	return nil
}

func (d *Daemon) persistKnown() error {
	ids := make([]string, 0, len(d.known))
	for id := range d.known {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return os.WriteFile(d.knownPath(), raw, 0o644)
}

func (d *Daemon) feedDashboard() {
	for id := range d.known {
		item, err := d.db.Load(id)
		if err != nil {
			continue
		}
		d.dash.Items().Upsert(dashboard.Item{ID: item.ID, State: item.State, Version: item.Version})
	}
}

func (d *Daemon) Tick(now time.Time) (bool, error) {
	items, err := d.source.Poll(d.lastPoll, 100)
	if err != nil {
		return false, err
	}
	didWork := false
	for _, intake := range items {
		if intake.ID == "" {
			return false, fmt.Errorf("%w: intake without id", ErrDaemon)
		}
		if d.known[intake.ID] {
			continue
		}
		if _, err := d.db.Load(intake.ID); err == nil {
			d.known[intake.ID] = true
			continue
		} else if !errors.Is(err, journal.ErrNotFound) {
			return false, err
		}
		reason := strings.TrimSpace(intake.Title)
		if reason == "" {
			reason = "github intake"
		}
		cmd := workflow.Command{
			ID:              workflow.CommandID(fmt.Sprintf("daemon-intake-%d", now.UnixNano())),
			AggregateID:     intake.ID,
			ExpectedVersion: 0,
			ActorID:         "actor/automation",
			Type:            workflow.CommandSubmitWork,
			Reason:          reason,
		}
		decision := (gatekeeper.Policy{}).Decide(gatekeeper.Context{
			Actor: gatekeeper.ActorAutomation, Profile: gatekeeper.ProfileStandard, ProjectMinimum: gatekeeper.ProfilePrototype,
		}, cmd)
		if !decision.Allowed {
			return false, fmt.Errorf("%w: intake denied (%s)", ErrDaemon, decision.Code)
		}
		if _, err := d.db.Apply(now, cmd); err != nil {
			return false, err
		}
		d.known[intake.ID] = true
		didWork = true
	}
	if err := d.persistKnown(); err != nil {
		return false, err
	}
	d.lastPoll = now
	d.feedDashboard()
	return didWork, nil
}
```

NOTE on command IDs: `now.UnixNano()` collides if two items submit within the same nanosecond tick on coarse clocks; the journal also dedupes by existence check before submit, so a collision can only occur across distinct items in one tick. Append a per-tick counter to be safe: `fmt.Sprintf("daemon-intake-%d-%d", now.UnixNano(), n)`.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/daemon -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/daemon/daemon.go internal/daemon/daemon_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/daemon/daemon.go internal/daemon/daemon_test.go
git commit -m "feat(daemon): open tick persist feed close"
```

### Task 3: Run the lifecycle and ship cmd/harnessd

**Files:**
- Create: `internal/daemon/run.go`, `internal/daemon/run_test.go`, `cmd/harnessd/main.go`

**Interfaces:**
- Consumes: `Open`/`Tick`/`Close` from Task 2.
- Produces: `Run(ctx, cfg, source) error` (fail fast at boot, resilient ticks, graceful
  shutdown), `main` with `--config` and `--check-config` (exit 0/1).

- [ ] **Step 1: Write the failing run test**

```go
package daemon

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestRunServesAndShutsDown(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(root)
	cfg.PollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg, &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}}) }()

	deadline := time.Now().Add(10 * time.Second)
	var addr string
	for {
		select {
		case err := <-done:
			t.Fatalf("run exited early: %v", err)
		default:
		}
		// The listener address is exposed for tests; poll until serving.
		if addr == "" {
			time.Sleep(20 * time.Millisecond)
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("daemon never served")
		}
		req, _ := http.NewRequest("GET", "http://"+addr+"/", nil)
		_ = req
		break
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown hung")
	}
	// Journal closed cleanly: reopening works.
	db, err := reopen(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_ = db
}
```

NOTE: this sketch is awkward (the polling loop is muddled). When writing the file,
do it cleanly: expose the bound address via `Daemon.Addr()` (captured from the real
`net.Listener`), start `Run` in a goroutine, wait for `Addr()` to become non-empty
with a deadline, assert `GET /` with the bearer token returns 200 and the ticked item
renders, then cancel and assert `Run` returns nil plus the DB reopens. `reopen` is just
`journal.Open` on the daemon DB path — reach it through a tiny exported helper
`DBPath(cfg) string` so tests do not duplicate the path layout.

So the `Run` implementation must: `net.Listen("tcp", cfg.BindAddr)`, serve
`dash.Handler()` on it, record the listener addr where `Addr()` can read it
(guard with mutex/atomic since `Run` owns the Daemon internally — simplest: `Run`
creates the Daemon via `Open`, then serves; expose address through a package-level
hook is ugly. Cleaner: split `Run` into `Serve(ctx, cfg, source, ready chan<- string)`
used by both `main` (ready=nil) and tests. When ready != nil, send the listener addr
once serving starts.)

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/daemon -run 'TestRunServesAndShutsDown' -count=1`

Expected: FAIL because `Run` (and `Serve`/`DBPath`) are undefined.

- [ ] **Step 3: Implement Run/Serve and cmd/harnessd**

```go
// internal/daemon/run.go
package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"
)

func DBPath(cfg Config) string {
	return filepath.Join(harnessDir(cfg.WorkspaceRoot), "harness.db")
}

func Run(ctx context.Context, cfg Config, source IntakeSource) error {
	return Serve(ctx, cfg, source, nil)
}

func Serve(ctx context.Context, cfg Config, source IntakeSource, ready chan<- string) error {
	d, err := Open(cfg, source)
	if err != nil {
		return err
	}
	defer d.Close()

	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		return err
	}
	if ready != nil {
		ready <- listener.Addr().String()
	}
	server := &http.Server{Handler: d.Dashboard().Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()

	if _, err := d.Tick(time.Now()); err != nil {
		_ = server.Close()
		return err
	}
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
			return nil
		case now := <-ticker.C:
			_, _ = d.Tick(now)
		}
	}
}
```

```go
// cmd/harnessd/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/marceloamoreno87/workflow-dev-template/internal/daemon"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "harnessd:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("harnessd", flag.ContinueOnError)
	configPath := fs.String("config", ".harness/daemon.yaml", "daemon config file")
	check := fs.Bool("check-config", false, "validate config and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := daemon.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if *check {
		fmt.Println("ok")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return daemon.Run(ctx, cfg, daemon.EmptySource())
}
```

NOTE: `LoadConfig` with a relative `--config` resolves against the process cwd
(standard behavior, document in `--help` text via the flag usage string).

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/daemon -count=1 && go build ./...`

Expected: PASS and clean build of `cmd/harnessd`.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/daemon/run.go internal/daemon/run_test.go cmd/harnessd/main.go && go test ./...`

Expected: PASS.

```bash
git add internal/daemon/run.go internal/daemon/run_test.go cmd/harnessd/main.go
git commit -m "feat(daemon): run lifecycle and harnessd entrypoint"
```

### Task 4: Prove the skeleton serves real projections

**Files:**
- Create: `internal/daemon/flow_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete skeleton from Tasks 1-3.
- Produces: executable evidence that boot serves ticked projections over loopback HTTP
  with auth, plus the plan index and verification entries.

- [ ] **Step 1: Add the flow test**

```go
package daemon

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestSkeletonServesProjections(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(root)
	cfg.PollInterval = 50 * time.Millisecond

	ready := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, cfg, &fakeSource{items: []IntakeItem{{ID: "owner/repo#7"}}}, ready)
	}()

	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		t.Fatalf("serve exited early: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("daemon never served")
	}
	get := func(path, token string) (int, string) {
		req, _ := http.NewRequest("GET", "http://"+addr+path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(raw)
	}
	if code, _ := get("/", ""); code != http.StatusUnauthorized {
		t.Fatalf("anonymous should be 401, got %d", code)
	}
	code, body := get("/", cfg.Token)
	if code != http.StatusOK {
		t.Fatalf("authed index should be 200, got %d", code)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if contains(body, "owner/repo#7") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("projection never served:\n%s", body)
		}
		time.Sleep(50 * time.Millisecond)
		code, body = get("/", cfg.Token)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown hung")
	}
}
```

NOTE: `contains` is `strings.Contains`; import `strings`.

- [ ] **Step 2: Run the flow test**

Run: `go test ./internal/daemon -run 'TestSkeletonServesProjections' -count=1`

Expected: PASS (initial tick submits before serving readiness is signaled… wait:
`ready` fires right after `Listen`, before the initial `Tick`. The test polls the
index until the item appears, so ordering is safe. Keep the poll loop as written.)

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean builds of `cmd/harness` and `cmd/harnessd`.

Run: `go run ./cmd/harnessd --check-config` against a temp config proving exit 0,
and against a broken one proving exit 1. (Manual evidence for the notes; the flag
parsing itself is covered by `run()` being thin — keep `main` untested directly.)

- [ ] **Step 4: Mark Increment 17 verified and commit**

In `docs/implementation-plan.md`, add Increment 17 to a v2 section of the plan index
and a verification checklist. Do not mark it complete until the commands above pass
on master.

```bash
git add internal/daemon/flow_test.go docs/implementation-plan.md
git commit -m "test(daemon): verify skeleton serves projections"
```
