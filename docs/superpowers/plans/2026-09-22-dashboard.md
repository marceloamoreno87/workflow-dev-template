# Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve an operator dashboard bound to loopback only, rendering Work Item projections through embedded Go templates and accepting version-bound Commands behind bearer auth, same-origin checks, and a strict content security policy.

**Architecture:** `dashboard` owns server construction with loopback enforcement, auth/CSRF/CSP middleware, projection view models, embedded templates, and command-intake validation over HTTP; it never opens non-loopback listeners and never applies commands to any store. It may import `internal/workflow` types plus the standard library. All other modules stay untouched with no new imports. Daemon wiring, session cookies, live projection feeds, and journal application belong to later increments and are explicitly out of scope.

**Tech Stack:** Go 1.27.1 standard library only (`net/http`, `net/http/httptest`, `html/template`, `embed`, `crypto/subtle`, `crypto/sha256`, table-driven tests)

**Spec:** `docs/architecture.md` (dashboard binds only to loopback, Go templates with embedded assets; projections are disposable), `docs/adr/0013-provide-cli-and-a-loopback-dashboard.md`, `docs/adr/0024-protect-the-loopback-dashboard.md` (loopback bind, authenticated session, CSRF, Origin/Host validation, strict CSP), `docs/adr/0025-render-the-dashboard-in-go.md`, `docs/contracts.md` (command envelope fields)

## Global Constraints

- Use Go 1.27.1.
- Standard library plus `internal/workflow` types only; do not add dependencies.
- Use the canonical terms from `CONTEXT.md` (Dashboard, Harness Workspace, Work Item, Command, Action, Gate); never write task or ticket for a Work Item.
- `NewServer` refuses any bind address that is not a loopback IP; tests prove `0.0.0.0` and external hostnames are rejected while `127.0.0.1` and `::1` are accepted.
- Every route requires `Authorization: Bearer <token>` with a ≥16-byte operator token compared via SHA-256 digest plus constant-time comparison; failures are 401 with no hint about which part was wrong.
- Mutating routes require an `Origin` header exactly matching the request host (`http://` or `https://` + `r.Host`); missing or foreign origins get 403. This is CSRF defense for browser clients on top of bearer auth.
- Every response carries `Content-Security-Policy: default-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'`; templates contain no inline scripts or styles.
- The dashboard validates and echoes Commands (202) but never applies them; application to the journal is daemon wiring (later increment). Dashboard-originated commands always carry the operator actor.
- Item IDs render escaped; the list is sorted by ID for deterministic output.
- Unit tests use `httptest` servers only (loopback by construction); no live listener ever opens in tests.
- Do not add generic repository, provider, manager, service, utils, or common packages; new code lives in `internal/dashboard` (plus its embedded `templates/`) only.

---

### Task 1: Enforce loopback, auth, Origin, and CSP

**Files:**
- Create: `internal/dashboard/server.go`
- Test: `internal/dashboard/server_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `Config struct`, `NewServer(cfg Config) (*Server, error)`, `Handler() http.Handler`, middleware chain auth → origin → CSP headers, sentinel `ErrDashboard`.

- [ ] **Step 1: Write the failing server test**

```go
package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func validConfig() Config {
	return Config{BindAddr: "127.0.0.1:0", Token: "operator-token-at-least-16"}
}

func TestRejectNonLoopbackBind(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{"0.0.0.0:8080", ":8080", "example.com:80", "not-an-addr", ""} {
		if _, err := NewServer(Config{BindAddr: addr, Token: "operator-token-at-least-16"}); err == nil {
			t.Fatalf("expected rejection for %q, got none", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080", "localhost:8080"} {
		if _, err := NewServer(Config{BindAddr: addr, Token: "operator-token-at-least-16"}); err != nil {
			t.Fatalf("expected acceptance for %q: %v", addr, err)
		}
	}
}

func TestAuthAndSecurityHeaders(t *testing.T) {
	srv, err := NewServer(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	res, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
	req, _ := http.NewRequest("GET", server.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", res.StatusCode)
	}
	req, _ = http.NewRequest("GET", server.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if csp := res.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("weak CSP: %q", csp)
	}
}

func TestOriginCheckOnMutations(t *testing.T) {
	srv, err := NewServer(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	post := func(origin string) int {
		req, _ := http.NewRequest("POST", server.URL+"/api/commands", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	host := strings.TrimPrefix(server.URL, "http://")
	if got := post("http://" + host); got == http.StatusForbidden {
		t.Fatal("same-origin POST was rejected")
	}
	if got := post(""); got != http.StatusForbidden {
		t.Fatalf("missing origin should be 403, got %d", got)
	}
	if got := post("http://evil.example"); got != http.StatusForbidden {
		t.Fatalf("foreign origin should be 403, got %d", got)
	}
}
```

NOTE: `TestAuthAndSecurityHeaders` and `TestOriginCheckOnMutations` must not use `t.Parallel` (none does — keep it that way) because they open ports; parallel is still fine for port tests, but sequential keeps logs clean. More importantly, `NewServer` in these tests never calls `ListenAndServe`, so no live listener opens: `httptest.NewServer` provides the loopback listener. `localhost:8080` must resolve to loopback — accept the literal hostname `localhost` plus loopback IPs in validation.

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/dashboard -run 'TestRejectNonLoopbackBind|TestAuthAndSecurityHeaders|TestOriginCheckOnMutations' -count=1`

Expected: FAIL because `Config`, `NewServer`, `Server.Handler`, and `ErrDashboard` are undefined.

- [ ] **Step 3: Implement the guarded server**

```go
// internal/dashboard/server.go
package dashboard

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

var ErrDashboard = errors.New("invalid dashboard configuration")

const cspValue = "default-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

type Config struct {
	BindAddr string
	Token    string
	Store    *Store
}

type Server struct {
	tokenHash [32]byte
	mux       *http.ServeMux
}

func loopbackHost(addr string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	if host == "" {
		return "", fmt.Errorf("empty host")
	}
	if strings.EqualFold(host, "localhost") {
		return host, nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("not loopback")
	}
	return host, nil
}

func NewServer(cfg Config) (*Server, error) {
	if _, err := loopbackHost(cfg.BindAddr); err != nil {
		return nil, fmt.Errorf("%w: bind %q: %v", ErrDashboard, cfg.BindAddr, err)
	}
	if len([]byte(cfg.Token)) < 16 {
		return nil, fmt.Errorf("%w: operator token too short", ErrDashboard)
	}
	store := cfg.Store
	if store == nil {
		store = NewStore()
	}
	s := &Server{tokenHash: sha256.Sum256([]byte(cfg.Token)), mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /items/{id}", s.handleItem)
	s.mux.HandleFunc("POST /api/commands", s.handleCommand)
	_ = store
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.withCSP(s.withAuth(s.mux))
}

func (s *Server) withCSP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", cspValue)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	got := r.Header.Get("Authorization")
	if !strings.HasPrefix(got, prefix) {
		return false
	}
	sum := sha256.Sum256([]byte(strings.TrimPrefix(got, prefix)))
	return subtle.ConstantTimeCompare(sum[:], s.tokenHash[:]) == 1
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
			origin := r.Header.Get("Origin")
			if origin != "http://"+r.Host && origin != "https://"+r.Host {
				http.Error(w, "foreign origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
```

NOTE: `handleIndex`, `handleItem`, `handleCommand`, `Store`, and `NewStore` arrive in Tasks 2–3. When writing `server.go` in Task 1, either stub the handlers minimally (index returns 200 empty page; item returns 404; command returns 501) and evolve them in later tasks, or write the full file once with forward references and add the missing pieces in Tasks 2–3 before running green. The plan's Steps 4–5 expect PASS at the end of Task 1, so include minimal handler stubs in Task 1 and replace them in Tasks 2–3.

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/dashboard -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/dashboard/server.go internal/dashboard/server_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/dashboard/server.go internal/dashboard/server_test.go
git commit -m "feat(dashboard): enforce loopback auth and origin"
```

### Task 2: Render projections through embedded templates

**Files:**
- Create: `internal/dashboard/store.go`, `internal/dashboard/views.go`, `internal/dashboard/templates/list.html`, `internal/dashboard/templates/detail.html`
- Test: `internal/dashboard/views_test.go`

**Interfaces:**
- Consumes: `workflow.State`/`WorkItemID`/`Version` types.
- Produces: `Item struct`, `Store` with `Upsert`/`Get`/`List`, `handleIndex` rendering the sorted list, `handleItem` rendering one item or 404, all content escaped.

- [ ] **Step 1: Write the failing views test**

```go
package dashboard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func testServer(t *testing.T, items ...Item) (*Server, *httptest.Server) {
	t.Helper()

	srv, err := NewServer(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		srv.Items().Upsert(item)
	}
	server := httptest.NewServer(srv.Handler())
	t.Cleanup(server.Close)
	return srv, server
}

func authedGet(t *testing.T, url string) (int, string) {
	t.Helper()

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(raw)
}

func TestIndexListsItemsSorted(t *testing.T) {
	_, server := testServer(t,
		Item{ID: "owner/repo#9", State: workflow.StateDone, Version: 5},
		Item{ID: "owner/repo#10", State: workflow.StateInbox, Version: 1},
	)
	code, body := authedGet(t, server.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	first := strings.Index(body, "owner/repo#10")
	second := strings.Index(body, "owner/repo#9")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("items not sorted:\n%s", body)
	}
	if !strings.Contains(body, "inbox") || !strings.Contains(body, "done") {
		t.Fatalf("states missing:\n%s", body)
	}
}

func TestDetailRendersAndEscapes(t *testing.T) {
	_, server := testServer(t, Item{ID: "<script>alert(1)</script>", State: workflow.StateTriage, Version: 2})
	code, body := authedGet(t, server.URL+"/items/"+urlPathEscape("<script>alert(1)</script>"))
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("unescaped content:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("escaped content missing:\n%s", body)
	}
	code, _ = authedGet(t, server.URL+"/items/owner%2Frepo%23123")
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}
```

NOTE: `urlPathEscape` is `net/url.PathEscape`; import it in the test. `srv.Items()` accessor and `Item`/`Store` arrive with this task. Detail route uses the Go 1.22 `GET /items/{id}` pattern with `r.PathValue("id")`.

Templates (no inline scripts or styles, plain HTML):

list.html:
```html
<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Work Items</title></head>
<body>
<h1>Work Items</h1>
<table>
<tr><th>ID</th><th>State</th><th>Version</th></tr>
{{range .}}<tr><td>{{.ID}}</td><td>{{.State}}</td><td>{{.Version}}</td></tr>{{end}}
</table>
</body>
</html>
```

detail.html:
```html
<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Work Item {{.ID}}</title></head>
<body>
<h1>{{.ID}}</h1>
<p>State: {{.State}}</p>
<p>Version: {{.Version}}</p>
</body>
</html>
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/dashboard -run 'TestIndexListsItemsSorted|TestDetailRendersAndEscapes' -count=1`

Expected: FAIL because `Item`, `Store`, `Items()`, and the handlers are undefined or stubbed.

- [ ] **Step 3: Implement the store, embedded templates, and handlers**

```go
// internal/dashboard/store.go
package dashboard

import (
	"sort"
	"sync"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

type Item struct {
	ID      workflow.WorkItemID
	State   workflow.State
	Version workflow.Version
}

type Store struct {
	mu    sync.RWMutex
	items map[workflow.WorkItemID]Item
}

func NewStore() *Store {
	return &Store{items: map[workflow.WorkItemID]Item{}}
}

func (s *Store) Upsert(item Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
}

func (s *Store) Get(id workflow.WorkItemID) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	return item, ok
}

func (s *Store) List() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Item, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
```

```go
// internal/dashboard/views.go
package dashboard

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

//go:embed templates/*.html
var templateFiles embed.FS

var listTemplate = template.Must(template.ParseFS(templateFiles, "templates/list.html"))

var detailTemplate = template.Must(template.ParseFS(templateFiles, "templates/detail.html"))

func (s *Server) Items() *Store {
	return s.store
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := listTemplate.Execute(w, s.store.List()); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

func (s *Server) handleItem(w http.ResponseWriter, r *http.Request) {
	item, ok := s.store.Get(workflow.WorkItemID(r.PathValue("id")))
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := detailTemplate.Execute(w, item); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}
```

Wire `store` into `Server` (replace the Task 1 `_ = store` line with a real field) and replace the Task 1 handler stubs.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/dashboard -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/dashboard/store.go internal/dashboard/views.go internal/dashboard/views_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/dashboard/store.go internal/dashboard/views.go internal/dashboard/views_test.go internal/dashboard/templates/list.html internal/dashboard/templates/detail.html
git commit -m "feat(dashboard): render projections from templates"
```

### Task 3: Validate command intake without applying

**Files:**
- Create: `internal/dashboard/command.go`
- Test: `internal/dashboard/command_test.go`

**Interfaces:**
- Consumes: `workflow` command-type allowlist.
- Produces: `handleCommand` accepting `POST /api/commands` with `{aggregateId, expectedVersion, type, reason}`, always stamping the operator actor, returning 202 with the normalized command or 400/422.

- [ ] **Step 1: Write the failing command test**

```go
package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func postCommand(t *testing.T, url, body string) (int, map[string]any) {
	t.Helper()

	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://"+strings.TrimPrefix(url, "http://"))
	trimmed := strings.TrimPrefix(url, "http://")
	_ = trimmed
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

NOTE: the Origin header above is overcomplicated and the `trimmed` lines are dead code; when writing the file, compute the origin cleanly: parse the test server URL once (`host := strings.TrimPrefix(server.URL, "http://")`) and set `Origin: http://<host>`. The `handleCommand` only compares against `r.Host`, so any test origin equal to `http://` + server host passes.

```go
func TestCommandIntake(t *testing.T) {
	_, server := testServer(t)
	host := strings.TrimPrefix(server.URL, "http://")

	post := func(body string) (int, map[string]any) {
		t.Helper()
		req, _ := http.NewRequest("POST", server.URL+"/api/commands", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://"+host)
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

	code, doc := post(`{"aggregateId":"owner/repo#123","expectedVersion":4,"type":"begin_triage","reason":""}`)
	if code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %v", code, doc)
	}
	if doc["actorId"] != "actor/operator" || doc["aggregateId"] != "owner/repo#123" {
		t.Fatalf("unexpected echo: %v", doc)
	}
	for _, body := range []string{
		`{}`,
		`{"aggregateId":"","expectedVersion":4,"type":"begin_triage"}`,
		`{"aggregateId":"owner/repo#123","expectedVersion":4,"type":"teleport"}`,
		`{"aggregateId":"owner/repo#123","expectedVersion":4,"type":"cancel"}`,
		`not json`,
	} {
		if code, _ := post(body); code != http.StatusBadRequest && code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 4xx for %q, got %d", body, code)
		}
	}
}
```

NOTE: `cancel` without a reason must fail (workflow requires reasons for block/cancel/reject/fail). The handler enforces the same reason rule as `internal/workflow`: `block`, `cancel`, `reject_work`, `fail_deployment` require non-empty reasons; respond 422 for a well-formed but semantically rejected command and 400 for malformed JSON/fields.

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/dashboard -run 'TestCommandIntake' -count=1`

Expected: FAIL because `handleCommand` is still the Task 1 stub (501) or undefined.

- [ ] **Step 3: Implement command intake validation**

```go
// internal/dashboard/command.go
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

var intakeTypes = map[workflow.CommandType]bool{
	workflow.CommandSubmitWork:            true,
	workflow.CommandBeginTriage:           true,
	workflow.CommandAuthorizeWork:         true,
	workflow.CommandRejectWork:            true,
	workflow.CommandBlock:                 true,
	workflow.CommandCancel:                true,
	workflow.CommandBeginSpec:             true,
	workflow.CommandBeginImplementation:   true,
	workflow.CommandApproveSpec:           true,
	workflow.CommandSubmitReview:          true,
	workflow.CommandRequestChanges:        true,
	workflow.CommandApprovePR:             true,
	workflow.CommandBeginDeploy:           true,
	workflow.CommandMarkDeploymentHealthy: true,
	workflow.CommandAcceptFeature:         true,
	workflow.CommandCompleteRollout:       true,
	workflow.CommandFailDeployment:        true,
	workflow.CommandRetryDeployment:       true,
	workflow.CommandResolveBlock:          true,
}

var reasonRequired = map[workflow.CommandType]bool{
	workflow.CommandBlock:          true,
	workflow.CommandCancel:         true,
	workflow.CommandRejectWork:     true,
	workflow.CommandFailDeployment: true,
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var doc struct {
		AggregateID     string `json:"aggregateId"`
		ExpectedVersion uint64 `json:"expectedVersion"`
		Type            string `json:"type"`
		Reason          string `json:"reason"`
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "unreadable body", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		http.Error(w, "malformed command", http.StatusBadRequest)
		return
	}
	cmdType := workflow.CommandType(doc.Type)
	if !intakeTypes[cmdType] {
		http.Error(w, "unknown command type", http.StatusBadRequest)
		return
	}
	aggregate := strings.TrimSpace(doc.AggregateID)
	if aggregate == "" || utf8.RuneCountInString(aggregate) > 256 {
		http.Error(w, "aggregate required", http.StatusBadRequest)
		return
	}
	if utf8.RuneCountInString(doc.Reason) > 2000 {
		http.Error(w, "reason too long", http.StatusBadRequest)
		return
	}
	if reasonRequired[cmdType] && strings.TrimSpace(doc.Reason) == "" {
		http.Error(w, "reason required", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, `{"aggregateId":%q,"expectedVersion":%d,"type":%q,"reason":%q,"actorId":"actor/operator","status":"accepted-for-review"}`+"\n",
		aggregate, doc.ExpectedVersion, string(cmdType), doc.Reason)
}
```

NOTE: the sketch uses `io` without importing it; add `"io"` to the imports when writing the file.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/dashboard -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/dashboard/command.go internal/dashboard/command_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/dashboard/command.go internal/dashboard/command_test.go
git commit -m "feat(dashboard): validate command intake"
```

### Task 4: Prove the guarded dashboard flow end-to-end

**Files:**
- Create: `internal/dashboard/flow_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete guarded dashboard from Tasks 1-3.
- Produces: executable evidence that the full chain (auth → origin → CSP → views → intake) holds in one server, plus the plan index and verification entries.

- [ ] **Step 1: Add the flow test**

```go
package dashboard

import (
	"net/http"
	"strings"
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestGuardedFlow(t *testing.T) {
	_, server := testServer(t, Item{ID: "owner/repo#123", State: workflow.StateReviewing, Version: 7})
	host := strings.TrimPrefix(server.URL, "http://")

	get := func(path, token string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest("GET", server.URL+path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res
	}
	if res := get("/", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous index: %d", res.StatusCode)
	}
	res := get("/", "operator-token-at-least-16")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("authed index: %d", res.StatusCode)
	}
	if csp := res.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("CSP missing: %q", csp)
	}
	res.Body.Close()

	post := func(body, origin string) int {
		t.Helper()
		req, _ := http.NewRequest("POST", server.URL+"/api/commands", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if got := post(`{"aggregateId":"owner/repo#123","expectedVersion":7,"type":"approve_pr","reason":""}`, "http://"+host); got != http.StatusAccepted {
		t.Fatalf("same-origin approve: %d", got)
	}
	if got := post(`{"aggregateId":"owner/repo#123","expectedVersion":7,"type":"approve_pr","reason":""}`, "http://evil.example"); got != http.StatusForbidden {
		t.Fatalf("foreign origin: %d", got)
	}
}
```

- [ ] **Step 2: Run the flow test**

Run: `go test ./internal/dashboard -run 'TestGuardedFlow' -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean build of `cmd/harness`.

- [ ] **Step 4: Mark Increment 13 verified and commit**

In `docs/implementation-plan.md`, add Increment 13 to the plan index and a verification checklist below Increment 12. Do not mark it complete until the commands above pass on master.

```bash
git add internal/dashboard/flow_test.go docs/implementation-plan.md
git commit -m "test(dashboard): verify guarded dashboard flow"
```
