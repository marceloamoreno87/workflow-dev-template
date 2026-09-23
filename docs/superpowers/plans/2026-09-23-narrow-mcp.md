# Narrow MCP Server Implementation Plan (Increment 19)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agent Threads talk to the daemon through one stdio JSON-RPC server exposing
exactly six tools — three reads and three validated mutations — bound at construction
to one Work Item and one bearer token, with cross-item calls rejected and secrets
nowhere in the protocol.

**Architecture:** `mcp` owns JSON-RPC framing, session auth, tool descriptors, argument
validation, scope enforcement, and dispatch against a `Backend` interface; it never
touches the network, the filesystem, or any daemon store directly. It may import
nothing outside the standard library. All other modules stay untouched with no new
imports. Daemon-side `Backend` wiring (journal, bundles, gate runners, command intake)
arrives with the human-surface increment; tests use a fake backend plus call counters
proving invalid calls never reach it.

**Tech Stack:** Go 1.27.1 standard library only (`encoding/json`, `crypto/subtle`,
`crypto/sha256`, `bufio`, `io`, stdio wiring in `cmd/harness-mcp`, table-driven tests).

**Spec:** `docs/superpowers/plans/2026-09-22-harness-organism.md` (Increment 19),
`docs/adr/0029-expose-harness-capabilities-through-narrow-mcp-tools.md`
(per-Execution authenticated tools; no shell/SQL/Docker/GitHub/credentials; mutations
become validated Commands through the same Gatekeeper), `docs/security.md`
(external prose is data).

## Global Constraints

- Use Go 1.27.1. Standard library only.
- Use the canonical terms from `CONTEXT.md` (Agent Thread, Agent Role, Work Item, Spec,
  Gate, Command, Knowledge Proposal); never write agent session, job, task, or ticket.
- Exactly six tools exist: `read_work`, `get_spec`, `search_knowledge`, `run_gate`,
  `request_gate`, `propose_knowledge`. A regression test asserts the listed set equals
  this set and names the forbidden ones (`exec_shell`, `sql`, `docker`, `github`,
  `coolify`, `keyring`) as absent.
- One server instance serves one Work Item: the item id and token bind at construction;
  any `workItem` argument differing from the bound id fails closed before the backend
  is touched.
- The bearer token arrives once via `initialize` params, verifies by SHA-256 plus
  constant-time comparison, and binds the stdio session; every `tools/*` call before a
  successful `initialize` fails; failures carry no hints about which half was wrong.
- Request lines cap at 1 MiB, call arguments at 64 KiB, `run_gate` output at 64 KiB
  with a truncation flag; oversized input is rejected, never truncated silently.
- Mutating tools validate fully in the MCP layer (gate-kind allowlist, reason bounds,
  proposal shape) and return validated envelopes/ids; they apply nothing themselves.
- `cmd/harness-mcp` reads the token from `HARNESS_MCP_TOKEN` (never argv), the item
  from `--work-item`, and serves stdio; missing token is fatal before reading input.
- Tests drive `Handle` directly and `Serve` over pipes; no sockets, no network.
- Do not add generic repository, provider, manager, service, utils, or common packages;
  new code lives in `internal/mcp` and `cmd/harness-mcp` only.

---

### Task 1: Frame JSON-RPC, authenticate, enumerate tools

**Files:**
- Create: `internal/mcp/server.go`
- Test: `internal/mcp/server_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `Backend` interface (full six-method shape, fake in tests),
  `NewServer(token, workItem string, backend Backend)`, `(Server).Handle(raw) []byte`,
  `(Server).Serve(r, w) error`, JSON-RPC error codes, sentinel `ErrMCP`.

- [ ] **Step 1: Write the failing transport test**

```go
package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func testBackend() *fakeBackend {
	return &fakeBackend{}
}

func TestInitializeBindsSession(t *testing.T) {
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", testBackend())

	if res := srv.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)); res == nil {
		t.Fatal("expected auth error, got notification treatment")
	} else if !strings.Contains(string(res), "not authenticated") {
		t.Fatalf("expected auth gate, got %s", res)
	}
	raw := srv.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"wrong"}}`))
	if !strings.Contains(string(raw), "auth failed") {
		t.Fatalf("expected auth failure, got %s", raw)
	}
	raw = srv.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`))
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["result"]; !ok {
		t.Fatalf("expected result, got %s", raw)
	}
	raw = srv.Handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	doc = nil
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	result, _ := doc["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 6 {
		t.Fatalf("expected 6 tools, got %s", raw)
	}
}

func TestToolsetIsClosed(t *testing.T) {
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", testBackend())
	srv.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`))
	raw := srv.Handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))

	names := map[string]bool{}
	var doc struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, tool := range doc.Result.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"read_work", "get_spec", "search_knowledge", "run_gate", "request_gate", "propose_knowledge"} {
		if !names[want] {
			t.Fatalf("missing tool %q", want)
		}
	}
	for _, banned := range []string{"exec_shell", "sql", "docker", "github", "coolify", "keyring"} {
		if names[banned] {
			t.Fatalf("forbidden tool %q listed", banned)
		}
	}
}

func TestProtocolErrors(t *testing.T) {
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", testBackend())

	if raw := srv.Handle([]byte(`not json`)); !strings.Contains(string(raw), "-32700") {
		t.Fatalf("expected parse error, got %s", raw)
	}
	auth := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`
	srv.Handle([]byte(auth))
	if raw := srv.Handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"teleport","params":{}}`)); !strings.Contains(string(raw), "-32601") {
		t.Fatalf("expected method-not-found, got %s", raw)
	}
	huge := `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{"pad":"` + strings.Repeat("x", 1024*1024) + `"}}`
	if raw := srv.Handle(huge); !strings.Contains(string(raw), "too large") {
		t.Fatal("expected size rejection")
	}
}
```

NOTE: `fakeBackend` (six methods, canned data, call counters) is defined once in
`backend_test.go` and shared by all test files in the package.

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/mcp -run 'TestInitializeBindsSession|TestToolsetIsClosed|TestProtocolErrors' -count=1`

Expected: FAIL (`NewServer`, `Handle`, `fakeBackend` undefined).

- [ ] **Step 3: Implement transport, auth, and the tool registry**

```go
// internal/mcp/server.go
package mcp

import (
	"bufio"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrMCP = errors.New("mcp request failed")

const maxLineBytes = 1024 * 1024

const maxArgumentsBytes = 64 * 1024

type Backend interface {
	ReadWork(workItem string) (WorkView, error)
	GetSpec(workItem string) (SpecDoc, error)
	SearchKnowledge(query string, limit int) ([]KnowledgeHit, error)
	RunGate(workItem, gate string) (GateResult, error)
	RequestGate(workItem, kind, reason string) (string, error)
	ProposeKnowledge(conceptID, title, body string, sources []string, reason string) error
}

type WorkView struct {
	ID      string
	State   string
	Version uint64
}

type SpecDoc struct {
	Ref  string
	Body string
}

type KnowledgeHit struct {
	ID      string
	Title   string
	Snippet string
}

type GateResult struct {
	Gate      string
	ExitCode  int
	Output    string
	Truncated bool
}

type tool struct {
	name        string
	description string
	schema      map[string]any
	call        func(s *Server, args map[string]any) (any, *rpcError)
}

type Server struct {
	tokenHash [32]byte
	workItem  string
	backend   Backend
	authed    bool
	tools     []tool
}

func NewServer(token, workItem string, backend Backend) *Server {
	s := &Server{tokenHash: sha256.Sum256([]byte(token)), workItem: workItem, backend: backend}
	s.tools = []tool{
		{name: "read_work", description: "Read the bound work item state.", schema: schemaFor("workItem"), call: callReadWork},
		{name: "get_spec", description: "Fetch the spec for the bound work item.", schema: schemaFor("workItem"), call: callGetSpec},
		{name: "search_knowledge", description: "Search project knowledge.", schema: searchSchema()}, 
		{name: "run_gate", description: "Run one declared gate for the bound work item.", schema: schemaFor("workItem", "gate"), call: callRunGate},
		{name: "request_gate", description: "Request a human gate decision.", schema: schemaFor("workItem", "kind", "reason"), call: callRequestGate},
		{name: "propose_knowledge", description: "Stage a knowledge proposal.", schema: schemaFor("conceptId", "title", "body", "sources", "reason"), call: callProposeKnowledge},
	}
	return s
}
```

Full dispatch (`Handle`, `Serve`, error codes -32700/-32601/-32602/-32001, `initialize`
verification, notifications returning nil) plus the `call*` handlers land across Tasks
1–3: Task 1 implements framing, auth, `tools/list`, and stub handlers returning
"not implemented" errors for the six calls; Tasks 2–3 replace the stubs. The Task 1
tests above pass with stubs in place (list works, calls need auth first anyway).

Schema helpers: `schemaFor` builds `{"type":"object","required":[...],"properties":{...}}`
with string-typed properties (and `sources` as string array); keep them static and
test-agnostic.

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/mcp -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/mcp/server.go internal/mcp/server_test.go internal/mcp/backend_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/mcp/server.go internal/mcp/server_test.go internal/mcp/backend_test.go
git commit -m "feat(mcp): frame json-rpc and bind sessions"
```

### Task 2: Serve reads with scope enforcement

**Files:**
- Create: `internal/mcp/tools.go` (or extend server.go — prefer `tools.go` for handlers)
- Test: `internal/mcp/tools_test.go`

**Interfaces:**
- Consumes: `Backend` + dispatch from Task 1.
- Produces: working `read_work`, `get_spec`, `search_knowledge` with bound-item scope
  checks happening before any backend call (proven by fake counters).

- [ ] **Step 1: Write the failing reads test**

```go
package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func authedServer(t *testing.T, backend *fakeBackend) *Server {
	t.Helper()

	srv := NewServer("execution-token-at-least-16", "owner/repo#1", backend)
	srv.Handle([]byte(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`))
	return srv
}

func call(t *testing.T, srv *Server, id int, name, args string) map[string]any {
	t.Helper()

	raw := srv.Handle([]byte(`{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`))
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestReadWork(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "read_work", `{"workItem":"owner/repo#1"}`)
	result, _ := doc["result"].(map[string]any)
	if result["id"] != "owner/repo#1" || result["state"] != "implementing" {
		t.Fatalf("unexpected work view: %v", doc)
	}
	doc = call(t, srv, 2, "read_work", `{"workItem":"owner/repo#2"}`)
	if _, ok := doc["error"]; !ok {
		t.Fatalf("cross-item read allowed: %v", doc)
	}
	if backend.reads != 1 {
		t.Fatalf("scope check must precede backend: reads=%d", backend.reads)
	}
}

func TestGetSpec(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "get_spec", `{"workItem":"owner/repo#1"}`)
	result, _ := doc["result"].(map[string]any)
	if result["ref"] != "owner/repo#1" || !strings.Contains(result["body"].(string), "health") {
		t.Fatalf("unexpected spec: %v", doc)
	}
}

func TestSearchKnowledge(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "search_knowledge", `{"query":"freeze","limit":5}`)
	result, _ := doc["result"].(map[string]any)
	hits, _ := result["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("unexpected hits: %v", doc)
	}
	doc = call(t, srv, 2, "search_knowledge", `{"query":""}`)
	if _, ok := doc["error"]; !ok {
		t.Fatalf("empty query allowed: %v", doc)
	}
}
```

NOTE: `itoa` helper (int → string) and the fake's canned views (`implementing`,
spec body containing "health", one hit for "freeze") are defined in `backend_test.go`.
`limit` clamps to 1..20 in the handler (backend receives the clamped value; assert
`backend.lastLimit == 5` in the test when writing the file).

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/mcp -run 'TestReadWork|TestGetSpec|TestSearchKnowledge' -count=1`

Expected: FAIL (stub handlers return "not implemented").

- [ ] **Step 3: Implement the read handlers**

Scope check helper: `scopeWorkItem(s, args) (string, *rpcError)` extracting the string
`workItem` field and comparing to `s.workItem` before touching the backend. Query
validation: trimmed length 1..500. Limit: default 10, clamp 1..20.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/mcp -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/mcp/tools.go internal/mcp/tools_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/mcp/tools.go internal/mcp/tools_test.go
git commit -m "feat(mcp): serve reads with scope"
```

### Task 3: Validate mutations and ship the binary

**Files:**
- Create: `cmd/harness-mcp/main.go`
- Test: `internal/mcp/mutate_test.go`

**Interfaces:**
- Consumes: dispatch from Task 1.
- Produces: working `run_gate` (declared-gate allowlist, 64 KiB cap), `request_gate`
  (kind allowlist `merge|production-deploy|client-acceptance`, reason 1..2000),
  `propose_knowledge` (id/title/body/sources/reason bounds); `harness-mcp` stdio main.

- [ ] **Step 1: Write the failing mutation test**

```go
package mcp

import (
	"strings"
	"testing"
)

func TestRunGate(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "run_gate", `{"workItem":"owner/repo#1","gate":"test"}`)
	result, _ := doc["result"].(map[string]any)
	if result["gate"] != "test" || result["exitCode"] != float64(0) {
		t.Fatalf("unexpected gate result: %v", doc)
	}
	doc = call(t, srv, 2, "run_gate", `{"workItem":"owner/repo#1","gate":"teleport"}`)
	if _, ok := doc["error"]; !ok {
		t.Fatalf("undeclared gate allowed: %v", doc)
	}
	if backend.gates["teleport"] {
		t.Fatal("undeclared gate reached backend")
	}
	// 100 KiB of fake output must arrive capped with its flag.
	doc = call(t, srv, 3, "run_gate", `{"workItem":"owner/repo#1","gate":"spam"}`)
	result, _ = doc["result"].(map[string]any)
	out, _ := result["output"].(string)
	if len(out) > 64*1024 || result["truncated"] != true {
		t.Fatalf("output not capped: len=%d %v", len(out), doc)
	}
	if !strings.HasPrefix(out, "spam") {
		t.Fatalf("output mangled: %q", out[:20])
	}
}

func TestRequestGate(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "request_gate", `{"workItem":"owner/repo#1","kind":"merge","reason":"gates green"}`)
	result, _ := doc["result"].(map[string]any)
	if result["commandId"] != "mcp-cmd-1" || result["status"] != "queued" {
		t.Fatalf("unexpected request: %v", doc)
	}
	for _, args := range []string{
		`{"workItem":"owner/repo#1","kind":"deploy-everything","reason":"x"}`,
		`{"workItem":"owner/repo#1","kind":"merge","reason":""}`,
		`{"workItem":"owner/repo#2","kind":"merge","reason":"x"}`,
	} {
		doc := call(t, srv, 2, "request_gate", args)
		if _, ok := doc["error"]; !ok {
			t.Fatalf("invalid request allowed: %s", args)
		}
	}
	if backend.requests != 1 {
		t.Fatalf("validation must precede backend: requests=%d", backend.requests)
	}
}

func TestProposeKnowledge(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "propose_knowledge", `{"conceptId":"deploy-freeze","title":"Deploy Freeze","body":"Freeze Fridays.","sources":["owner/repo#1"],"reason":"operator review"}`)
	if _, ok := doc["result"]; !ok {
		t.Fatalf("valid proposal rejected: %v", doc)
	}
	doc = call(t, srv, 2, "propose_knowledge", `{"conceptId":"deploy-freeze","title":"t","body":"b","sources":[],"reason":"r"}`)
	if _, ok := doc["error"]; !ok {
		t.Fatal("unsourced proposal allowed")
	}
	if backend.proposals != 1 {
		t.Fatalf("validation must precede backend: proposals=%d", backend.proposals)
	}
}
```

NOTE: the fake's `RunGate("spam")` returns 100 KiB output; `RequestGate` mints
`mcp-cmd-<n>`; counters `reads/specs/searches/gates map/requests/proposals` prove
validation precedes backend access.

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/mcp -run 'TestRunGate|TestRequestGate|TestProposeKnowledge' -count=1`

Expected: FAIL (stub handlers).

- [ ] **Step 3: Implement mutation handlers and `cmd/harness-mcp`**

Gate allowlist mirrors the stack catalog gate names
(`format|lint|typecheck|test|build|smoke`); request kinds
(`merge|production-deploy|client-acceptance`); proposal bounds mirror the knowledge
module (id slug, title 1..200, body 1..100000 runes, ≥1 source, reason 1..2000).

```go
// cmd/harness-mcp/main.go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/marceloamoreno87/workflow-dev-template/internal/mcp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "harness-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	fs := flag.NewFlagSet("harness-mcp", flag.ContinueOnError)
	workItem := fs.String("work-item", "", "bound work item id")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *workItem == "" {
		return fmt.Errorf("work item required")
	}
	token := os.Getenv("HARNESS_MCP_TOKEN")
	if token == "" {
		return fmt.Errorf("HARNESS_MCP_TOKEN required")
	}
	return mcp.NewServer(token, *workItem, mcp.StubBackend()).Serve(os.Stdin, os.Stdout)
}
```

NOTE: `mcp.StubBackend()` does not exist — the binary needs a Backend and the real
daemon wiring lands later. Decision when writing the file: `cmd/harness-mcp` serves a
read-only diagnostic backend (`mcp.DiagBackend()`: reads return empty views, mutations
return "not wired" errors) so the binary is exercisable end to end without pretending
to mutate. Daemon wiring replaces it. Name and document accordingly.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/mcp -count=1 && go build ./...`

Expected: PASS and clean build including `cmd/harness-mcp`.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/mcp/ cmd/harness-mcp/main.go && go test ./...`

Expected: PASS.

```bash
git add internal/mcp/ cmd/harness-mcp/main.go
git commit -m "feat(mcp): validate mutations and ship binary"
```

### Task 4: Prove a full authenticated session over pipes

**Files:**
- Create: `internal/mcp/session_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete server from Tasks 1-3.
- Produces: executable evidence of initialize → list → read → request → propose over
  one `Serve` pipe, plus a second session proving a wrong token learns nothing, plus
  the plan index and verification entries.

- [ ] **Step 1: Add the session test**

```go
package mcp

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

func TestFullSession(t *testing.T) {
	backend := &fakeBackend{}
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", backend)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_work","arguments":{"workItem":"owner/repo#1"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"request_gate","arguments":{"workItem":"owner/repo#1","kind":"merge","reason":"gates green"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"propose_knowledge","arguments":{"conceptId":"c","title":"t","body":"b","sources":["s"],"reason":"r"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
	}, "\n") + "\n"

	var output strings.Builder
	if err := srv.Serve(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 responses (notification is silent), got %d:\n%s", len(lines), output.String())
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[4]), &last); err != nil {
		t.Fatal(err)
	}
	if _, ok := last["result"]; !ok {
		t.Fatalf("proposal failed: %s", lines[4])
	}
	if backend.requests != 1 || backend.proposals != 1 {
		t.Fatalf("backend not driven: %+v", backend)
	}
}

func TestWrongTokenLearnsNothing(t *testing.T) {
	backend := &fakeBackend{}
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", backend)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"wrong"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_work","arguments":{"workItem":"owner/repo#1"}}}`,
	}, "\n") + "\n"

	var output strings.Builder
	if err := srv.Serve(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		if strings.Contains(line, `"result"`) {
			t.Fatalf("unauthenticated session returned data: %s", line)
		}
	}
	if backend.reads != 0 {
		t.Fatal("backend touched without auth")
	}
}
```

- [ ] **Step 2: Run the session test**

Run: `go test ./internal/mcp -run 'TestFullSession|TestWrongTokenLearnsNothing' -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean builds of `cmd/harness`, `cmd/harnessd`, and `cmd/harness-mcp`.

- [ ] **Step 4: Mark Increment 19 verified and commit**

In `docs/implementation-plan.md`, add Increment 19 to the v2 plan index and a
verification checklist. Do not mark it complete until the commands above pass on master.

```bash
git add internal/mcp/session_test.go docs/implementation-plan.md
git commit -m "test(mcp): verify authenticated sessions"
```
