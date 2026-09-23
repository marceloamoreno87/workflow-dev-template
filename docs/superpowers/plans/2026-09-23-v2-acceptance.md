# v2 Acceptance Implementation Plan (Increment 22)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the daemon-owned chain end to end — GitHub-shaped poller → intake →
triage → human authorize → product → implement → host gates → review → loop done —
across a daemon restart, with secret canaries planted in the environment and asserted
absent from every artifact the run produces, plus an operator runbook for replaying
the full v1 scenario against real systems.

**Architecture:** no new production code is expected; this increment is executable
evidence plus docs. If the acceptance test exposes a missing seam (unexported helper,
unreachable state), add the smallest test-visible surface in `internal/daemon` with a
comment marking it as acceptance support. All other modules stay untouched.

**Tech Stack:** Go 1.27.1 standard library only, `httptest` GitHub double, stub
`codex`/toolchain binaries, `t.Setenv` canaries (no `t.Parallel` anywhere near them).

**Spec:** `docs/superpowers/plans/2026-09-22-harness-organism.md` (Increment 22),
`docs/security.md` (credentials outside worktrees, prompts, logs, SQLite).

## Global Constraints

- Use Go 1.27.1. Standard library plus existing internal modules only.
- Use the canonical terms from `CONTEXT.md` (Work Item, Agent Thread, Execution,
  Gate, Evidence, Human Gate); never write session, job, task, or ticket.
- Canaries live ONLY in process environment (`GH_TOKEN`, `GITHUB_TOKEN`,
  `COOLIFY_TOKEN`, plus one `HARNESS_CANARY` control); no canary appears in any
  fixture file, title, or reason by construction — so any occurrence in an artifact
  is a leak, no exceptions.
- The sweep covers: raw `harness.db` bytes, `daemon-known.json`, `daemon-loops.json`,
  `telegram-offset` (if created), the stub's captured `argv.log`, `stdin.txt`
  (the exact model prompt), per-binary `env-*.txt` dumps, dashboard HTTP bodies, and
  every error string observed during the run.
- The stub `codex` script gains one line dumping its environment to
  `$stub_dir/env-codex.txt`; gate stubs in this test dump theirs to `env-go.txt`.
  These dumps exist solely to prove deny-by-default environments end to end.
- The run asserts exact workflow versions at each stage (submit=1 … submit_review=5,
  reviewer approval moves nothing) so duplicates or skips fail loudly.
- Restart happens between the implement tick and the reviewer tick with the daemon
  fully closed and reopened; the run completes with no duplicate commands.
- The runbook (`docs/acceptance.md`) is operator procedure, not code: config, service
  install, issue→authorize→observe→approve→deploy→expose→accept→rollout→debt→OKF,
  each step naming the gate command that produces its evidence. It must not claim
  automation that does not exist yet (keyring, Projects sync, container gates, MCP
  daemon wiring are listed as manual steps with their future increment).
- Do not add generic repository, provider, manager, service, utils, or common packages.

---

### Task 1: Run the acceptance scenario with canaries

**Files:**
- Create: `internal/daemon/acceptance_test.go`
- Modify: `internal/daemon/fixture_test.go` (env dump line in the stub codex script)

**Interfaces:**
- Consumes: `pollerAdapter` (github_test.go), `authorizeAsHuman`/`mustLoad`
  (pipeline_test.go), `seedReviewing` not needed.
- Produces: `TestAcceptanceScenario` — the whole daemon-owned chain in one test.

- [ ] **Step 1: Write the failing acceptance test**

```go
package daemon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/githublive"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestAcceptanceScenario(t *testing.T) {
	// No t.Parallel: canaries ride process environment.
	t.Setenv("GH_TOKEN", "canary-gh-token-001")
	t.Setenv("GITHUB_TOKEN", "canary-github-token-002")
	t.Setenv("COOLIFY_TOKEN", "canary-coolify-token-003")
	t.Setenv("HARNESS_CANARY", "canary-control-004")
	canaries := []string{"canary-gh-token-001", "canary-github-token-002", "canary-coolify-token-003", "canary-control-004"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":78,"title":"Add health endpoint","body":"x","state":"open","user":{"login":"octocat"},"updated_at":"2026-09-23T00:00:00Z"}]`))
	}))
	defer server.Close()

	root := fixtureWorkspace(t)
	bin := stubBin(t)
	writeStub(t, bin, "gofmt", "#!/bin/sh\nexit 0\n")
	writeStub(t, bin, "go", "#!/bin/sh\nenv | sort > \"$(dirname \"$0\")/env-go.txt\"\nexit 0\n")

	client, err := githublive.NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := githublive.NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(testConfig(root), &pollerAdapter{poller: poller, owner: "owner", repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var observedErrs []string

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	authorizeAsHuman(t, d, "owner/repo#78", 2)
	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := mustLoad(t, d, "owner/repo#78"); got.State != workflow.StateReviewing || got.Version != 5 {
		t.Fatalf("implement leg incomplete: %#v", got)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(testConfig(root), &pollerAdapter{poller: poller, owner: "owner", repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	if _, err := reopened.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	final := mustLoad(t, reopened, "owner/repo#78")
	if final.State != workflow.StateReviewing || final.Version != 5 {
		t.Fatalf("restart broke the chain: %#v", final)
	}
	done, ok := reopened.LoopState("owner/repo#78")
	if !ok || done.Stage != "done" {
		t.Fatalf("loop not done: %#v %v", done, ok)
	}

	// Canary sweep over every artifact the run produced.
	artifacts := map[string]string{}
	dbBytes, err := os.ReadFile(DBPath(testConfig(root)))
	if err != nil {
		t.Fatal(err)
	}
	artifacts["harness.db"] = string(dbBytes)
	for _, name := range []string{"daemon-known.json", "daemon-loops.json"} {
		raw, err := os.ReadFile(filepath.Join(root, ".harness", name))
		if err != nil {
			t.Fatal(err)
		}
		artifacts[name] = string(raw)
	}
	for _, name := range []string{"argv.log", "stdin.txt", "env-codex.txt", "env-go.txt"} {
		raw, err := os.ReadFile(filepath.Join(bin, name))
		if err != nil {
			t.Fatal(err)
		}
		artifacts[name] = string(raw)
	}
	dash := httptest.NewServer(reopened.Dashboard().Handler())
	defer dash.Close()
	req, _ := http.NewRequest("GET", dash.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	artifacts["dashboard"] = string(body)
	for name, content := range artifacts {
		for _, canary := range canaries {
			if strings.Contains(content, canary) {
				t.Fatalf("canary %q leaked into %s", canary, name)
			}
		}
	}
	_ = observedErrs
}
```

NOTE: drop the dead `observedErrs` lines when writing the file (errors already fail
the test at each step; there is nothing to collect). `DBPath(testConfig(root))`
recomputes the path without opening — fine. The dashboard GET needs no Origin (GET
is not mutating). `authorizeAsHuman` at version 2 assumes submit(1)+triage(2) from
tick one — matches the pipeline tests.

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/daemon -run 'TestAcceptanceScenario' -count=1`

Expected: FAIL (no `env-codex.txt` — the stub does not dump env yet — plus the test
itself is new).

- [ ] **Step 3: Add the env dump line and run green**

Append to the stub codex script in `fixture_test.go`, right after the argv log line:
`env | sort > "$stub_dir/env-codex.txt"`. Re-run: PASS expected. If anything else
fails, fix the seam (not the test) unless the test encodes a wrong expectation —
document which.

- [ ] **Step 4: Format and commit**

Run: `gofmt -w internal/daemon/acceptance_test.go internal/daemon/fixture_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/daemon/acceptance_test.go internal/daemon/fixture_test.go
git commit -m "test(daemon): run acceptance scenario with canaries"
```

### Task 2: Write the operator acceptance runbook

**Files:**
- Create: `docs/acceptance.md`

**Interfaces:** none (prose). Content: prerequisites (Go toolchain, git, daemon config
+ token files, systemd), install (`harnessd --install-service`), register/validate a
fixture project, open a GitHub issue, watch poller intake on the dashboard, authorize
via Telegram, observe role ticks (product→implement→gates→review) with Evidence,
approve the PR challenge-gated, verify merge preservation + release immutability,
deploy disabled via broker, targeted exposure, contributor acceptance, progressive
rollout, 14-day flag removal, OKF proposal review — each step naming the gate command
and where its evidence lives. Explicit manual-step callouts for keyring tokens,
Projects v2 sync, webhooks, containerized gates, and MCP daemon wiring.

- [ ] **Step 1: Write `docs/acceptance.md`.**
- [ ] **Step 2: Commit.**

```bash
git add docs/acceptance.md
git commit -m "docs: add operator acceptance runbook"
```

### Task 3: Close out v2 organism work

**Files:**
- Modify: `docs/implementation-plan.md`, `README.md` (one line + runbook link)

**Interfaces:** none (prose).

- [ ] **Step 1: Mark Increment 22 verified; write the organism close-out note listing
  what the daemon now does unattended, what stays human-gated by design, and what
  remains future (keyring, Projects sync, webhooks, container gates, MCP wiring,
  promotion metrics). Update README's verification/runtime paragraph with the daemon
  run line and the runbook link.
- [ ] **Step 2: Run the full verification set** (`go test -race ./...`, `go vet ./...`,
  `go build ./...`).
- [ ] **Step 3: Commit.**

```bash
git add internal/daemon/flow_test.go docs/implementation-plan.md README.md
git commit -m "test(daemon): verify v2 acceptance and close organism"
```

NOTE: there is no `flow_test.go` change in this increment — the file list above is a
leftover; commit `internal/daemon/acceptance_test.go` if it is not already committed
in Task 1 (it is), so this commit is docs-only: `docs/implementation-plan.md`
`docs/acceptance.md` if amended, `README.md`.
