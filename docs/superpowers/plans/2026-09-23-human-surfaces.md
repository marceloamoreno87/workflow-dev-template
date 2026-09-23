# Human Surfaces Live Implementation Plan (Increment 21)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dashboard commands and Telegram updates move real Work Items: validated
dashboard intents commit through the Gatekeeper into the journal (202 only after
commit), and a Telegram poll loop ingests allowlisted commands — plain ones applied,
sensitive ones challenged — with offsets surviving restarts, plus a systemd unit that
installs the daemon as a user service.

**Architecture:** `dashboard` gains an `Apply` callback (set by the daemon; 501 when
absent) mapping apply outcomes to HTTP codes. `daemon` gains operator-command
application, Telegram configuration (non-secret block plus secret files), a Bot API
client user (`internal/telegram` grows `client.go`), and a poll/parse/challenge/apply
loop with a file-persisted offset. It may import `internal/dashboard`,
`internal/telegram` plus its existing modules and stdlib. All other modules stay
untouched with no new imports. Projects v2 status sync, keyring-backed secrets, and
webhook ingestion stay out of scope.

**Tech Stack:** Go 1.27.1 standard library only (`net/http`, `net/http/httptest`,
`encoding/json`, `os`, table-driven tests, one systemd unit template).

**Spec:** `docs/superpowers/plans/2026-09-22-harness-organism.md` (Increment 21),
`docs/security.md` (Telegram allowlist, five-minute challenge/message expiry),
`docs/adr/0021-route-telegram-through-the-gatekeeper.md`,
`docs/architecture.md` (`harnessd` as `systemd --user` service).

## Global Constraints

- Use Go 1.27.1. Standard library plus existing internal modules only.
- Use the canonical terms from `CONTEXT.md` (Human Gate, Actor, Command, Gate,
  Work Item, Acceptance Gate); never write session, job, task, or ticket.
- Dashboard `Apply` is nil-safe: without a daemon behind it the route stays 501 and
  every standalone dashboard test keeps passing unchanged.
- Applied outcomes map deterministically: commit → 202 with the new version; stale
  `expectedVersion` → 409 with the currently allowed actions; unknown type or bad
  shape → 400; legal shape that the state machine refuses → 422 (same reason rule as
  `workflow`: block/cancel/reject/fail require reasons).
- Dashboard-originated commands always carry `actor/operator`; allowlisted Telegram
  users act as operator in this single-operator increment (contributor actors arrive
  with multi-user work, explicitly out of scope).
- The Telegram cursor lives in `<workspace>/.harness/telegram-offset` (plain integer,
  daemon-owned like the other cursors); the journal schema stays frozen.
- Challenges are stateless: the issued text carries `challenge <id> <exp>` (MAC covers
  actor, gate, version, expiry); verification recomputes with constant-time compare and
  rejects expired windows, including daemon-offline-late arrivals. Secrets (bot token,
  challenge secret ≥16 bytes) arrive via files, never config values, URLs in errors, or
  logs; the Bot API path token is redacted from any recorded string.
- Poll failures never fail the tick (logged nowhere — no logging infra yet — just
  skipped with the cursor unadvanced); malformed updates are skipped individually while
  the batch offset still advances past them.
- Tests use `httptest` Bot doubles and never touch `api.telegram.org`.
- Do not add generic repository, provider, manager, service, utils, or common packages;
  new code lives in `internal/dashboard`, `internal/daemon`, `internal/telegram`,
  `cmd/harnessd`, and `systemd/user/harnessd.service` (template) only.

---

### Task 1: Apply dashboard commands through a callback

**Files:**
- Modify: `internal/dashboard/command.go`
- Test: `internal/dashboard/apply_test.go`

**Interfaces:**
- Consumes: existing intake validation from Increment 13.
- Produces: `CommandRequest struct`, `Applied struct`, `ApplyFunc type`,
  `Config.Apply`, outcome mapping 202/400/409/422/501, sentinel `ErrApplyConflict`
  and `ErrApplyRejected` for the daemon to return.

- [ ] **Step 1: Write the failing apply test**

```go
package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func postAuthed(t *testing.T, url, body string) (int, map[string]any) {
	t.Helper()

	host := strings.TrimPrefix(url, "http://")
	_ = host
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://"+strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://"))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	return res.StatusCode, doc
}
```

NOTE: the Origin construction above is convoluted leftovers; when writing the file,
compute `origin := url` scheme+host cleanly from the test server URL (the handler only
compares against `r.Host`, so `Origin: http://<host>` suffices — same helper shape as
the existing command/flow tests).

```go
func TestApplyCommits(t *testing.T) {
	srv, err := NewServer(Config{BindAddr: "127.0.0.1:0", Token: "operator-token-at-least-16", Apply: func(req CommandRequest) (Applied, error) {
		if req.Type != "begin_triage" {
			t.Errorf("unexpected type: %q", req.Type)
		}
		return Applied{Version: 1}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	code, doc := postAuthed(t, server.URL+"/api/commands", `{"aggregateId":"owner/repo#123","expectedVersion":0,"type":"begin_triage"}`)
	if code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %v", code, doc)
	}
	if doc["version"] != float64(1) || doc["actorId"] != "actor/operator" {
		t.Fatalf("unexpected echo: %v", doc)
	}
}

func TestApplyMapsErrors(t *testing.T) {
	newServer := func(apply ApplyFunc) *httptest.Server {
		srv, err := NewServer(Config{BindAddr: "127.0.0.1:0", Token: "operator-token-at-least-16", Apply: apply})
		if err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(srv.Handler())
		t.Cleanup(server.Close)
		return server
	}
	_ = newServer

	conflict, err := NewServer(Config{BindAddr: "127.0.0.1:0", Token: "operator-token-at-least-16", Apply: func(req CommandRequest) (Applied, error) {
		return Applied{}, ErrApplyConflict
	}})
	if err != nil {
		t.Fatal(err)
	}
	conflictServer := httptest.NewServer(conflict.Handler())
	defer conflictServer.Close()
	if code, _ := postAuthed(t, conflictServer.URL+"/api/commands", `{"aggregateId":"owner/repo#123","expectedVersion":0,"type":"begin_triage"}`); code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", code)
	}

	rejected, err := NewServer(Config{BindAddr: "127.0.0.1:0", Token: "operator-token-at-least-16", Apply: func(req CommandRequest) (Applied, error) {
		return Applied{}, ErrApplyRejected
	}})
	if err != nil {
		t.Fatal(err)
	}
	rejectedServer := httptest.NewServer(rejected.Handler())
	defer rejectedServer.Close()
	if code, _ := postAuthed(t, rejectedServer.URL+"/api/commands", `{"aggregateId":"owner/repo#123","expectedVersion":0,"type":"begin_triage"}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", code)
	}

	bare, err := NewServer(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	bareServer := httptest.NewServer(bare.Handler())
	defer bareServer.Close()
	if code, _ := postAuthed(t, bareServer.URL+"/api/commands", `{"aggregateId":"owner/repo#123","expectedVersion":0,"type":"begin_triage"}`); code != http.StatusNotImplemented {
		t.Fatalf("expected 501 without applier, got %d", code)
	}
}
```

NOTE: drop the dead `newServer` closure when writing the file (it is unused scaffolding
in this sketch).

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/dashboard -run 'TestApplyCommits|TestApplyMapsErrors' -count=1`

Expected: FAIL (no `Apply`, still 202-echo without applying).

- [ ] **Step 3: Implement the apply callback**

`CommandRequest{AggregateID string, ExpectedVersion uint64, Type workflow.CommandType,
Reason string}`; `Applied{Version uint64}`; `ApplyFunc func(CommandRequest)
(Applied, error)`; `Config.Apply ApplyFunc` (nil = standalone); sentinels
`ErrApplyConflict`, `ErrApplyRejected`. `handleCommand` keeps all Increment 13
validation, then: nil applier → 501; call → nil error → 202 with
`{aggregateId, expectedVersion: <request>, type, reason, actorId, version, status}` —
set `status` to `"applied"` (replacing the old `"accepted-for-review"` value now that
application is real when wired; standalone 501 path keeps no status). Conflict →
409 with `{..., "allowed": [...]}`? Allowed actions need workflow knowledge: the
daemon returns them inside the error? Errors are strings... Decision when writing:
`ErrApplyConflict` is returned wrapped with the allowed list rendered by the daemon
(`fmt.Errorf("%w: %s", ErrApplyConflict, strings.Join(allowed, ","))`), and the
handler splits on the first `": "` to populate `"allowed"`. Document the convention
in code. Rejected → 422. Anything else → 500 without detail.

Existing tests calling the route without `Apply` expect 202 with `accepted-for-review`
— update `command_test.go`/`flow_test.go` expectations in this commit (202 + applied
requires an applier; without one they now get 501). This is the intended semantic
promotion; keep the change visible in the commit.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/dashboard -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/dashboard/ && go test ./...`

Expected: PASS.

```bash
git add internal/dashboard/command.go internal/dashboard/apply_test.go internal/dashboard/command_test.go internal/dashboard/flow_test.go
git commit -m "feat(dashboard): apply commands through callback"
```

### Task 2: Apply operator commands and load Telegram config

**Files:**
- Create: `internal/daemon/apply.go`
- Test: `internal/daemon/apply_test.go`

**Interfaces:**
- Consumes: `dashboard.CommandRequest/Applied/ErrApply*` shapes.
- Produces: `(Daemon).ApplyOperatorCommand(req) (uint64, error)` (gatekeeper operator
  context + journal apply + conflict/allowed mapping); `TelegramConfig` load with
  secret files; `Config.Telegram *TelegramConfig` (nil = disabled).

- [ ] **Step 1: Write the failing apply test**

```go
package daemon

import (
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/dashboard"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestApplyOperatorCommand(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Tick(ctx(), time.Now()); err != nil {
		t.Fatal(err)
	}
	version, err := d.ApplyOperatorCommand(dashboard.CommandRequest{
		AggregateID: "owner/repo#1", ExpectedVersion: 1, Type: workflow.CommandAuthorizeWork,
	})
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("expected version 2, got %d", version)
	}
	if got := mustLoad(t, d, "owner/repo#1"); got.State != workflow.StateReady {
		t.Fatalf("not authorized: %#v", got)
	}
}

func TestApplyConflictListsAllowed(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	seedReady(t, d, "owner/repo#9")
	_, err = d.ApplyOperatorCommand(dashboard.CommandRequest{
		AggregateID: "owner/repo#9", ExpectedVersion: 0, Type: workflow.CommandBeginSpec,
	})
	if err == nil {
		t.Fatal("expected stale rejection, got none")
	}
	if !isConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}
}
```

NOTE: `isConflict` is `errors.Is(err, dashboard.ErrApplyConflict)` inline; the tick
in the first test leaves the item at triage/version 1 (intake submit + auto-triage in
one tick — Increment 18 behavior), so authorizing at expected version 1 is correct.
`seedReady` (advance_test.go) leaves version 3, so expecting stale at version 0 is
correct.

Telegram config test (same file or `telegram_test.go` — prefer a separate
`internal/daemon/telegram_test.go` in Task 3 with the client; config shape test here):

```go
func TestTelegramConfig(t *testing.T) {
	root := t.TempDir()
	// absent block → nil, disabled.
	cfg, err := LoadConfig(writeConfig(t, root, validBody(root)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Telegram != nil {
		t.Fatal("telegram should default disabled")
	}
}
```

Plus a valid-block case and a bad-secret case in Task 3's test file alongside the
client (config shape: `telegram: {baseURL, allowedUsers, allowedChats, botTokenFile,
challengeSecretFile}`).

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/daemon -run 'TestApplyOperatorCommand|TestApplyConflictListsAllowed|TestTelegramConfig' -count=1`

Expected: FAIL (`ApplyOperatorCommand`, `Config.Telegram` undefined).

- [ ] **Step 3: Implement operator apply and Telegram config**

```go
// internal/daemon/apply.go
package daemon

import (
	"fmt"
	"strings"

	"github.com/marceloamoreno87/workflow-dev-template/internal/dashboard"
	"github.com/marceloamoreno87/workflow-dev-template/internal/gatekeeper"
	"github.com/marceloamoreno87/workflow-dev-template/internal/journal"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
	"errors"
)

func (d *Daemon) ApplyOperatorCommand(req dashboard.CommandRequest) (uint64, error) {
	item, err := d.db.Load(workflow.WorkItemID(req.AggregateID))
	empty := workflow.WorkItem{}
	if err != nil && !errors.Is(err, journal.ErrNotFound) {
		return 0, err
	}
	if errors.Is(err, journal.ErrNotFound) {
		item = empty
	}
	cmd := workflow.Command{
		ID: commandID(), AggregateID: workflow.WorkItemID(req.AggregateID),
		ExpectedVersion: workflow.Version(req.ExpectedVersion),
		ActorID: "actor/operator", Type: req.Type, Reason: req.Reason,
	}
	decision := (gatekeeper.Policy{}).Decide(gatekeeper.Context{
		State: item.State, Actor: gatekeeper.ActorOperator,
		Profile: gatekeeper.ProfileStandard, ProjectMinimum: gatekeeper.ProfilePrototype,
	}, cmd)
	if !decision.Allowed {
		return 0, fmt.Errorf("%w: %s", dashboard.ErrApplyRejected, decision.Code)
	}
	events, err := d.db.Apply(time.Now(), cmd)
	if err != nil {
		if errors.Is(err, journal.ErrConflict) {
			allowed := (workflow.Workflow{}).Allowed(item)
			names := make([]string, 0, len(allowed))
			for _, a := range allowed {
				names = append(names, string(a))
			}
			return 0, fmt.Errorf("%w: %s", dashboard.ErrApplyConflict, strings.Join(names, ","))
		}
		return 0, err
	}
	last := events[len(events)-1]
	_ = last
	applied, err := d.db.Load(workflow.WorkItemID(req.AggregateID))
	if err != nil {
		return 0, err
	}
	return uint64(applied.Version), nil
}
```

NOTE: `time` import needed; drop the dead `last` lines when writing the file (return
the reloaded version directly). Clock: `time.Now()` at the edge is daemon-owned —
allowed (daemon is the designated clock reader since Increment 17).

Telegram config in `config.go`: `Telegram *TelegramConfig` with
`{BaseURL string, AllowedUsers, AllowedChats []int64, BotToken, ChallengeSecret string(unexported? no — exported fields, secrets in memory only)}`; yaml block `telegram:` with
`baseURL, allowedUsers, allowedChats, botTokenFile, challengeSecretFile`; absent block
→ nil; present → validate (http(s) URL, non-empty user+chat lists, files readable,
challenge secret ≥16 bytes, bot token non-empty) and load secrets.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/daemon -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/daemon/apply.go internal/daemon/apply_test.go internal/daemon/config.go && go test ./...`

Expected: PASS.

```bash
git add internal/daemon/apply.go internal/daemon/apply_test.go internal/daemon/config.go internal/daemon/config_test.go
git commit -m "feat(daemon): apply operator commands and telegram config"
```

### Task 3: Poll Telegram and challenge sensitive commands

**Files:**
- Create: `internal/telegram/client.go`
- Test: `internal/telegram/client_test.go`
- Create: `internal/daemon/telegram.go`
- Test: `internal/daemon/telegram_test.go`

**Interfaces:**
- Consumes: `telegram.ParseUpdate/ParseCommand/NewChallenge/VerifyChallenge`;
  daemon journal + offset file.
- Produces: `telegram.Client{BaseURL, Token}`, `GetUpdates(ctx, offset, timeoutSecs)
  ([]RawUpdate, error)`, `SendMessage(ctx, chatID, text) error` with token redaction;
  `daemon.pollTelegram(ctx, now) error` (offset load/advance, per-update handling,
  challenge issuance for sensitive intents lacking valid challenges).

- [ ] **Step 1: Write the failing client test**

```go
package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetUpdates(t *testing.T) {
	t.Parallel()

	var gotOffset string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/botstub-token/getUpdates") {
			t.Errorf("path leaks or wrong: %s", r.URL.Path)
		}
		gotOffset = r.URL.Query().Get("offset")
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":41,"message":{"message_id":7,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":"/changes owner/repo#1 3 tests","date":` + itoa(time.Now().Unix()) + `}}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	updates, err := client.GetUpdates(context.Background(), 40, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 41 {
		t.Fatalf("unexpected updates: %#v", updates)
	}
	if gotOffset != "40" {
		t.Fatalf("offset not sent: %q", gotOffset)
	}
}

func TestSendMessage(t *testing.T) {
	t.Parallel()

	var gotPath, gotText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var doc struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&doc)
		gotText = doc.Text
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendMessage(context.Background(), 2002, "hello"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/botstub-token/sendMessage" || gotText != "hello" {
		t.Fatalf("unexpected send: %s %q", gotPath, gotText)
	}
}

func TestClientRejects(t *testing.T) {
	t.Parallel()

	if _, err := NewClient("", "tok"); err == nil {
		t.Fatal("expected url rejection, got none")
	}
	if _, err := NewClient("https://api.example", ""); err == nil {
		t.Fatal("expected token rejection, got none")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"boom"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetUpdates(context.Background(), 0, 0); err == nil {
		t.Fatal("expected ok:false failure, got none")
	} else if strings.Contains(err.Error(), "s3cr3t") {
		t.Fatalf("secret leaked: %v", err)
	}
}
```

NOTE: imports `encoding/json`, `time`, `strconv` (`itoa` helper exists in update_test.go —
reuse it, do not redefine).

- [ ] **Step 2: Write the failing poll test**

```go
package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func telegramDouble(t *testing.T, updates string, sent *[]string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			_, _ = w.Write([]byte(`{"ok":true,"result":[` + updates + `]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			var doc struct {
				Text string `json:"text"`
			}
			_ = json.NewDecoder(r.Body).Decode(&doc)
			*sent = append(*sent, doc.Text)
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func telegramDaemon(t *testing.T, server *httptest.Server) *Daemon {
	t.Helper()

	root := t.TempDir()
	cfg := testConfig(root)
	cfg.Telegram = testTelegramConfig(t, server.URL)
	d, err := Open(cfg, &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestTelegramPlainCommandApplies(t *testing.T) {
	now := time.Now()
	update := `{"update_id":41,"message":{"message_id":7,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":"/changes owner/repo#1 0 needs tests","date":` + itoa(now.Unix()) + `}}`
	var sent []string
	server := telegramDouble(t, update, &sent)
	defer server.Close()

	d := telegramDaemon(t, server)
	seedReviewing(t, d, "owner/repo#1")
	if err := d.pollTelegram(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := mustLoad(t, d, "owner/repo#1"); got.State != workflow.StateChangesRequested {
		t.Fatalf("plain command not applied: %#v", got)
	}
	if len(sent) != 0 {
		t.Fatalf("no message expected, got %v", sent)
	}
	// Offset persisted: a second poll with the same double sees nothing new.
	var sent2 []string
	server2 := telegramDouble(t, "", &sent2)
	defer server2.Close()
	_ = server2
}

func TestTelegramSensitiveNeedsChallenge(t *testing.T) {
	now := time.Now()
	update := `{"update_id":42,"message":{"message_id":7,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":"/approve owner/repo#1 3","date":` + itoa(now.Unix()) + `}}`
	var sent []string
	server := telegramDouble(t, update, &sent)
	defer server.Close()

	d := telegramDaemon(t, server)
	seedReviewing(t, d, "owner/repo#1")
	if err := d.pollTelegram(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := mustLoad(t, d, "owner/repo#1"); got.State != workflow.StateReviewing {
		t.Fatalf("sensitive command applied without challenge: %#v", got)
	}
	if len(sent) != 1 || !strings.Contains(sent[0], "challenge") {
		t.Fatalf("challenge not offered: %v", sent)
	}
}
```

NOTE: several helpers are referenced but not defined here — define them when writing
the files: `testTelegramConfig(t, baseURL)` (writes secret files, returns
`*TelegramConfig`), `seedReviewing(t, d, id)` (journal path submit→triage→authorize→
begin_implementation→submit_review with matching versions/actors), `itoa` (exists in
telegram package tests, but daemon tests need their own — define locally in
`telegram_test.go`). The second-poll offset assertion is sketched loosely above;
when writing the file, assert properly: after the first poll, read the offset file
(`.harness/telegram-offset` == `42`), then repointing is unnecessary — instead close
and reopen the daemon against a double serving only update 41 (older id): the persisted
offset (42) filters it client-side... wait, Telegram server-side filters by offset;
reopening with the same double serving update 42 again would redeliver. Assert offset
file content == 42 and, after reopen with an empty double, no duplicate apply (journal
version unchanged). Write it that way.

Challenge acceptance test (valid challenge applies): construct the reason
`challenge <id> <exp> ship it` using `NewChallenge` directly in the test with the same
secret file content, then poll and assert applied with reason `ship it`. Include it.

- [ ] **Step 3: Run the tests and verify they fail**

Run: `go test ./internal/telegram ./internal/daemon -run 'TestGetUpdates|TestSendMessage|TestClientRejects|TestTelegramPlain|TestTelegramSensitive' -count=1`

Expected: FAIL (no `Client`, no `pollTelegram`, no `TelegramConfig` plumbing).

- [ ] **Step 4: Implement the Bot client and the poll loop**

`telegram/client.go`: `Client{baseURL, token}`, `NewClient` (http(s), no userinfo,
non-empty token), `GetUpdates(ctx, offset int64, timeoutSecs int) ([]RawUpdate, error)`
(GET `{base}/bot<token>/getUpdates?offset=&timeout=&limit=100`, envelope `{ok,
result, description}`, 1 MiB cap, secret-free errors — never include URL or body),
`RawUpdate{UpdateID int64, Message json.RawMessage}`, `SendMessage(ctx, chatID int64,
text string) error` (POST JSON `{chat_id, text}`, text 1..4096).

`daemon/telegram.go`: offset load/save (`.harness/telegram-offset`, plain int64,
missing → 0); `pollTelegram(ctx, now) error` (nil telegram config → nil immediately):
fetch with timeout 0; for each update: unmarshal message envelope via
`telegram.ParseUpdate(raw, telegram.Config{...from daemon cfg...}, now)` — ParseUpdate
takes the full update JSON including `update_id`, which it ignores; on parse error
(skip update, continue); `ParseCommand`; non-sensitive → `ApplyOperatorCommand`-equivalent
internal apply (same gatekeeper+journal path, actor operator) with reason; sensitive →
extract leading `challenge <id> <exp>` from reason: absent/malformed → issue
`NewChallenge(secret, "actor/operator", intent.Type, version, now)` + `SendMessage`
with the reply format and continue; present → `VerifyChallenge` → on success apply
with stripped reason, on failure skip (no message — avoid oracle loops; document).
Advance offset past every seen `update_id` (even skipped) and persist once per poll.

Internal apply shared with `ApplyOperatorCommand`: refactor it to take an actor
parameter? Dashboard stamps operator; telegram stamps operator too (single-operator).
Reuse `ApplyOperatorCommand` directly with the dashboard request shape (construct
`dashboard.CommandRequest` internally) — no refactor needed.

- [ ] **Step 5: Run the package tests**

Run: `go test ./internal/telegram ./internal/daemon -count=1`

Expected: PASS.

- [ ] **Step 6: Format and commit**

Run: `gofmt -w internal/telegram/client.go internal/telegram/client_test.go internal/daemon/telegram.go internal/daemon/telegram_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/telegram/client.go internal/telegram/client_test.go internal/daemon/telegram.go internal/daemon/telegram_test.go internal/daemon/config.go internal/daemon/config_test.go
git commit -m "feat(daemon): poll telegram with challenges"
```

### Task 4: Ship the systemd unit

**Files:**
- Create: `systemd/user/harnessd.service` (template with `@EXEC@`/`@CONFIG@` placeholders? No — generate concretely)
- Modify: `cmd/harnessd/main.go` (`--install-service`), `internal/daemon` `SystemdUnit` render + test
- Test: extend `internal/daemon` with `systemd_test.go`

**Interfaces:**
- Produces: `daemon.SystemdUnit(execPath, configPath string) string`,
  `harnessd --install-service [--config …]` writing
  `$HOME/.config/systemd/user/harnessd.service` (0644, MkdirAll parents) and printing
  the `systemctl --user daemon-reload` + `enable --now` hint.

- [ ] **Step 1: Write the failing unit test**

```go
package daemon

import (
	"strings"
	"testing"
)

func TestSystemdUnit(t *testing.T) {
	t.Parallel()

	unit := SystemdUnit("/usr/local/bin/harnessd", "/home/op/.harness/daemon.yaml")
	for _, want := range []string{
		"Description=Harness daemon",
		"ExecStart=/usr/local/bin/harnessd --config /home/op/.harness/daemon.yaml",
		"WantedBy=default.target",
		"Restart=on-failure",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
}
```

Plus a repo template file `systemd/user/harnessd.service` with the same content shape
(relative exec/config left for the installer to fill — mark clearly as template).

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/daemon -run 'TestSystemdUnit' -count=1`

Expected: FAIL (`SystemdUnit` undefined).

- [ ] **Step 3: Implement render + installer flag**

`SystemdUnit(execPath, configPath)` returns the unit text (validate non-empty inputs,
else return ""? — return error? Keep signature simple: pure string builder assuming
validated inputs; main validates flags first. Hmm, fail-closed is better: return
`(string, error)` rejecting empty paths. Adjust test to check error path too when
writing the file.)

`cmd/harnessd/main.go`: `--install-service` flag; when set, resolve exec path via
`os.Executable()`, config path absolute-ized, render, `MkdirAll ~/.config/systemd/user`,
write file, print hint, exit 0. `--check-config` and `--install-service` combine:
validate first when both... keep independent (install validates implicitly by loading?
No — install does not need a valid config file present; it only writes paths. Keep
independent, document.)

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/daemon -count=1 && go build ./...`

Expected: PASS and clean build.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/daemon/systemd.go internal/daemon/systemd_test.go cmd/harnessd/main.go && go test ./...`

Expected: PASS.

```bash
git add internal/daemon/systemd.go internal/daemon/systemd_test.go cmd/harnessd/main.go systemd/user/harnessd.service
git commit -m "feat(daemon): install systemd user service"
```

### Task 5: Prove surfaces move real items

**Files:**
- Create: `internal/daemon/surfaces_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete Tasks 1-4.
- Produces: executable evidence of dashboard POST → journal commit and telegram
  batch → mixed apply/challenge/skip with offset restart, plus index + verification.

- [ ] **Step 1: Add the flow tests**

```go
package daemon

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestDashboardPostCommits(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Tick(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(d.Dashboard().Handler())
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	req, _ := http.NewRequest("POST", server.URL+"/api/commands", strings.NewReader(`{"aggregateId":"owner/repo#1","expectedVersion":2,"type":"authorize_work"}`))
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://"+host)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", res.StatusCode, raw)
	}
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	if doc["version"] != float64(3) {
		t.Fatalf("unexpected echo: %v", doc)
	}
	if got := mustLoad(t, d, "owner/repo#1"); got.State != workflow.StateReady {
		t.Fatalf("journal not moved: %#v", got)
	}
}
```

Wait — the dashboard handler does not know the daemon applier yet: `Open` must wire
`dashboard.Config.Apply` to `d.ApplyOperatorCommand` (adapted to the dashboard
signature). Do that wiring in Task 2's implementation (it is part of "apply path":
`Open` sets `Apply` when constructing the dashboard server). Note it explicitly in
Task 2 Step 3 (already implied by "used by dashboard handler" — make it explicit when
writing the code).

```go
func TestTelegramBatchFlow(t *testing.T) {
	now := time.Now()
	plain := `{"update_id":51,"message":{"message_id":1,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":"/changes owner/repo#1 3 tests","date":` + itoa(now.Unix()) + `}}`
	stale := `{"update_id":52,"message":{"message_id":2,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":"/changes owner/repo#1 3 old","date":` + itoa(now.Add(-time.Hour).Unix()) + `}}`
	stranger := `{"update_id":53,"message":{"message_id":3,"from":{"id":9999},"chat":{"id":2002,"type":"private"},"text":"/changes owner/repo#1 3 evil","date":` + itoa(now.Unix()) + `}}`
	sensitive := `{"update_id":54,"message":{"message_id":4,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":"/approve owner/repo#1 3","date":` + itoa(now.Unix()) + `}}`
	var sent []string
	server := telegramDouble(t, strings.Join([]string{plain, stale, stranger, sensitive}, ","), &sent)
	defer server.Close()
```

NOTE: `telegramDouble` serves the same `updates` string for every `getUpdates` call —
a second poll would redeliver. The test asserts offset behavior instead: after the
poll, read `.harness/telegram-offset` (expect 54); journal shows one `request_changes`
(triaged→? wait — `request_changes` from which state? Seed the item to reviewing
first via `seedReviewing`, version 5? Seed path submit(1)→triage(2)→authorize(3)→
begin_implementation(4, operator? begin_implementation actor automation ok)→
submit_review(5, automation ok). Then `/changes … 3` has stale expectedVersion 3 vs
actual 5 → 409 conflict! Fix versions when writing the file: seed to reviewing
(version 5) and send expected version 5. Then plain applies → changes_requested v6;
stale/stranger skipped; sensitive challenged (sent has 1 challenge text); offset 54;
reopen with empty double → versions unchanged.

`itoa` for daemon tests: define once in `telegram_test.go` (Task 3) and reuse here.

- [ ] **Step 2: Run the flow tests**

Run: `go test ./internal/daemon -run 'TestDashboardPostCommits|TestTelegramBatchFlow' -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean builds of all three binaries.

- [ ] **Step 4: Mark Increment 21 verified and commit**

In `docs/implementation-plan.md`, add Increment 21 to the v2 plan index and a
verification checklist. Do not mark it complete until the commands above pass on master.

```bash
git add internal/daemon/surfaces_test.go docs/implementation-plan.md
git commit -m "test(daemon): verify live human surfaces"
```
