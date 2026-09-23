# GitHub Wiring Implementation Plan (Increment 20)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Read real GitHub Issues through a cursor poller that plugs directly into the
daemon as its `IntakeSource`, and open history-preserving PRs plus immutable releases
through a typed client whose every call is recorded — all against GitHub-shaped doubles
in tests and only against explicit operator credentials live.

**Architecture:** `githublive` owns the HTTP client, the issue poller (which implements
`daemon.IntakeSource`), PR open/merge with client-side merge-method enforcement,
release creation, commit listing with foreign-commit detection, and bounded rate-limit
retries; it never shells out and never contacts the network except through its
configured base URL. It may import `internal/daemon` (for the intake types only) plus
the standard library. All other modules stay untouched with no new imports — except the
`IntakeSource.Poll` signature, which gains a context so daemon cancellation interrupts
hung polls (mechanical update of `daemon` and its fakes inside Task 1). Keyring-backed
tokens, webhook ingestion, and Projects v2 status sync belong to later increments and
are explicitly out of scope.

**Tech Stack:** Go 1.27.1 standard library only (`net/http`, `net/http/httptest`,
`encoding/json`, `context`, table-driven tests).

**Spec:** `docs/superpowers/plans/2026-09-22-harness-organism.md` (Increment 20),
`docs/adr/0008-reconcile-github-by-polling.md` (cursor polling, deterministic resume),
`docs/adr/0007-authenticate-automation-as-a-github-app.md` (least-privilege tokens),
`docs/workflow.md` (merge preserving commits; approval/release as separate facts).

## Global Constraints

- Use Go 1.27.1. Standard library plus `internal/daemon` (intake types only).
- Use the canonical terms from `CONTEXT.md` (Work Item, Command, Gate, Human Gate,
  Execution); never write session, job, task, or ticket.
- Doubles speak real GitHub REST shapes (`/repos/{owner}/{repo}/issues`,
  `/pulls`, `/pulls/{n}/merge`, `/releases`) so swapping base URL plus token is the
  only production change; harness-invented paths are forbidden here.
- The token travels in one `Authorization: Bearer` header; URLs never carry userinfo;
  errors carry status codes, never tokens or bodies.
- `MergePR` enforces `method == "merge"` client-side and never calls the server
  otherwise; `merged: false` responses fail closed.
- Releases require strict semver tags and 40-hex commits (same patterns as delivery).
- `Poll` clamps limit 1..100, sends `since` only when non-zero, and maps only
  well-formed issues (positive number, non-empty title, open/closed state, parseable
  timestamp when present); anything else fails the poll closed.
- Retries cover 429 plus 5xx (never 4xx otherwise, never 501), at most 3 attempts,
  honoring `Retry-After` seconds capped at 1s and falling back to 50ms steps; every
  attempt counts in tests.
- Response bodies cap at 1 MiB; every call is context-bound.
- Live tests need `HARNESS_GITHUB_URL` plus `HARNESS_GITHUB_TOKEN` and skip otherwise.
- Do not add generic repository, provider, manager, service, utils, or common packages;
  new code lives in `internal/githublive` only (plus the `IntakeSource` signature edit).

---

### Task 1: Client, poller, and the context-carrying source

**Files:**
- Create: `internal/githublive/client.go`, `internal/githublive/poll.go`
- Test: `internal/githublive/client_test.go`, `internal/githublive/poll_test.go`
- Modify: `internal/daemon/daemon.go` (`IntakeSource.Poll` gains `ctx`),
  `internal/daemon/daemon_test.go` (fake signature), `internal/daemon/run.go` if needed

**Interfaces:**
- Consumes: `daemon.IntakeItem`/`daemon.IntakeSource` shapes.
- Produces: `Client struct`, `NewClient(baseURL, token)`, `TokenProvider` interface +
  `StaticToken`, `Poller struct`, `NewPoller(client, owner, repo)`,
  `(Poller).Poll(ctx, since, limit) ([]daemon.IntakeItem, error)` satisfying the source.

- [ ] **Step 1: Write the failing client/poller test**

```go
package githublive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRejectBadClients(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		baseURL string
		token   string
	}{
		{name: "empty url", baseURL: "", token: "tok"},
		{name: "no host", baseURL: "https://", token: "tok"},
		{name: "userinfo", baseURL: "https://user:pass@api.example", token: "tok"},
		{name: "empty token", baseURL: "https://api.example", token: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(tc.baseURL, tc.token); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestPollMapsIssues(t *testing.T) {
	t.Parallel()

	var gotSince, gotState string
	var gotPerPage int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/issues" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer stub-token" {
			t.Errorf("auth wrong: %q", r.Header.Get("Authorization"))
		}
		query := r.URL.Query()
		gotSince, gotState = query.Get("since"), query.Get("state")
		fmt.Sscanf(query.Get("per_page"), "%d", &gotPerPage)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
{"number":78,"title":"Add health endpoint","body":"Please.","state":"open","user":{"login":"octocat"},"updated_at":"2026-09-23T00:00:00Z"},
{"number":79,"title":"Closed item","body":null,"state":"closed","user":{"login":"octocat"},"updated_at":"2026-09-23T01:00:00Z"}
]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	since := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	items, err := poller.Poll(context.Background(), since, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "owner/repo#78" || items[0].Title != "Add health endpoint" {
		t.Fatalf("unexpected items: %#v", items)
	}
	if gotState != "all" || gotPerPage != 50 || !strings.Contains(gotSince, "2026-09-22") {
		t.Fatalf("query wrong: since=%q state=%q per_page=%d", gotSince, gotState, gotPerPage)
	}
}

func TestPollRejectsMalformed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":0,"title":"x","state":"open"}]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := poller.Poll(context.Background(), time.Now(), 10); err == nil {
		t.Fatal("expected malformed rejection, got none")
	}
}

func TestPollHidesSecrets(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "s3cr3t-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := poller.Poll(context.Background(), time.Now(), 10); err == nil {
		t.Fatal("expected failure, got none")
	} else if strings.Contains(err.Error(), "s3cr3t-token") {
		t.Fatalf("secret leaked: %v", err)
	}
}
```

NOTE: the test file needs `fmt` imported for `Sscanf` (or parse with `strconv.Atoi`;
use `strconv` when writing the file).

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/githublive -run 'TestRejectBadClients|TestPollMapsIssues|TestPollRejectsMalformed|TestPollHidesSecrets' -count=1`

Expected: FAIL (`NewClient`, `NewPoller`, `Poll` undefined).

- [ ] **Step 3: Implement client, poller, and the source migration**

```go
// internal/githublive/client.go
package githublive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrGitHub = errors.New("github request failed")

const responseCap = 1024 * 1024

type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

type staticToken string

func StaticToken(token string) TokenProvider {
	return staticToken(token)
}

func (s staticToken) Token(context.Context) (string, error) {
	if s == "" {
		return "", fmt.Errorf("%w: empty token", ErrGitHub)
	}
	return string(s), nil
}

type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewClient(baseURL, token string) (Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Client{}, fmt.Errorf("%w: base url needs http(s) with host", ErrGitHub)
	}
	if u.User != nil {
		return Client{}, fmt.Errorf("%w: credentials do not belong in urls", ErrGitHub)
	}
	if token == "" {
		return Client{}, fmt.Errorf("%w: token required", ErrGitHub)
	}
	return Client{baseURL: strings.TrimSuffix(u.Scheme + "://" + u.Host + u.Path, "/"), token: token, client: &http.Client{}}, nil
}
```

Plus shared `do(ctx, method, path, query, payload) ([]byte, Call, error)` with the
30s timeout envelope, status mapping (2xx ok; 429/5xx retryable marker), 1 MiB cap,
and secret-free errors; `Call{Method, Path, StatusCode}` returned for evidence.

```go
// internal/githublive/poll.go
package githublive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/daemon"
)

var ownerPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

type Poller struct {
	client Client
	owner  string
	repo   string
}

func NewPoller(client Client, owner, repo string) (*Poller, error) {
	if !ownerPattern.MatchString(owner) || !ownerPattern.MatchString(repo) {
		return nil, fmt.Errorf("%w: owner/repo", ErrGitHub)
	}
	return &Poller{client: client, owner: owner, repo: repo}, nil
}

func (p *Poller) Poll(ctx context.Context, since time.Time, limit int) ([]daemon.IntakeItem, error) {
	if limit <= 0 || limit > 100 {
		return nil, fmt.Errorf("%w: limit outside 1..100", ErrGitHub)
	}
	query := url.Values{}
	query.Set("state", "all")
	query.Set("per_page", strconv.Itoa(limit))
	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}
	body, _, err := p.client.get(ctx, "/repos/"+p.owner+"/"+p.repo+"/issues", query)
	if err != nil {
		return nil, err
	}
	var docs []struct {
		Number    int     `json:"number"`
		Title     string  `json:"title"`
		Body      *string `json:"body"`
		State     string  `json:"state"`
		UpdatedAt string  `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &docs); err != nil {
		return nil, fmt.Errorf("%w: decode issues", ErrGitHub)
	}
	items := make([]daemon.IntakeItem, 0, len(docs))
	for _, doc := range docs {
		title := strings.TrimSpace(doc.Title)
		if doc.Number <= 0 || title == "" {
			return nil, fmt.Errorf("%w: malformed issue", ErrGitHub)
		}
		if doc.State != "open" && doc.State != "closed" {
			return nil, fmt.Errorf("%w: issue state %q", ErrGitHub, doc.State)
		}
		if doc.UpdatedAt != "" {
			if _, err := time.Parse(time.RFC3339, doc.UpdatedAt); err != nil {
				return nil, fmt.Errorf("%w: issue timestamp", ErrGitHub)
			}
		}
		items = append(items, daemon.IntakeItem{
			ID:    daemon.WorkItemID(fmt.Sprintf("%s/%s#%d", p.owner, p.repo, doc.Number)),
			Title: title,
		})
	}
	return items, nil
}
```

Daemon migration in the same task: `IntakeSource.Poll` gains `ctx context.Context`;
`emptySource`, `fakeSource` (daemon_test), `Tick` internals, and `run.go` updated
mechanically. `daemon.WorkItemID` does not exist — the intake type lives in the
`daemon` package as `IntakeItem{ID workflow.WorkItemID}`; reference the ID type via
`workflow.WorkItemID` in `githublive` (import `internal/workflow`, not the whole
daemon package — lighter and cycle-free since daemon never imports githublive).
Adjust: `Poll` returns a local `Issue` struct; add a tiny `func (p *Poller) PollItems`
mapping? Decision when writing the file: `poll.go` defines `type Issue struct`
with the validated fields and a `func (p *Poller) Poll` returning `[]Issue`, plus
`daemon` gains a 10-line adapter in Task 4 (`func IntakeFromIssues(owner, repo string,
issues []githublive.Issue) []daemon.IntakeItem` would import githublive from daemon —
allowed, one direction only). Simpler still: keep the mapping inline in Task 4's flow
test? No — production needs it in non-test code. Final: `githublive.Issue` +
`daemon.IntakeFromIssues` adapter (Task 4), `Poller.Poll` returns `[]Issue`.
Rewrite the Task 1 test assertions accordingly when writing the files: assert
`issues[0].Number/Title`, not `IntakeItem`.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/githublive ./internal/daemon -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/githublive/ internal/daemon/ && go test ./...`

Expected: PASS.

```bash
git add internal/githublive/client.go internal/githublive/poll.go internal/githublive/client_test.go internal/githublive/poll_test.go internal/daemon/daemon.go internal/daemon/daemon_test.go internal/daemon/run.go
git commit -m "feat(github): poll issues as intake source"
```

### Task 2: Open history-preserving PRs

**Files:**
- Create: `internal/githublive/pullrequest.go`
- Test: `internal/githublive/pullrequest_test.go`

**Interfaces:**
- Consumes: `Client.do`/`Call` from Task 1.
- Produces: `NewPR struct`, `PR struct`, `(Client).OpenPR(ctx, NewPR) (PR, Call, error)`,
  `(Client).MergePR(ctx, number, method) (Merge, Call, error)` with merge-only enforcement.

- [ ] **Step 1: Write the failing PR test**

```go
package githublive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenPR(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != "POST" {
			t.Errorf("method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":12,"head":{"sha":"` + strings.Repeat("d", 40) + `"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	pr, call, err := client.OpenPR(context.Background(), "owner", "repo", NewPR{Base: "main", Head: "feat-x", Title: "Add x", Body: "Evidence attached."})
	if err != nil {
		t.Fatal(err)
	}
	if pr.Number != 12 || call.Method != "POST" || gotPath != "/repos/owner/repo/pulls" {
		t.Fatalf("unexpected open: %#v %#v %s", pr, call, gotPath)
	}
}

func TestMergeEnforcesMethod(t *testing.T) {
	t.Parallel()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/repos/owner/repo/pulls/12/merge" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"merged":true,"sha":"` + strings.Repeat("d", 40) + `"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"squash", "rebase", ""} {
		if _, _, err := client.MergePR(context.Background(), "owner", "repo", 12, method); err == nil {
			t.Fatalf("method %q reached the server", method)
		}
	}
	if calls != 0 {
		t.Fatal("non-merge method reached the server")
	}
	merge, _, err := client.MergePR(context.Background(), "owner", "repo", 12, "merge")
	if err != nil {
		t.Fatal(err)
	}
	if !merge.Merged {
		t.Fatal("merge not reported")
	}
}

func TestMergeRefusesUnmerged(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"merged":false,"message":"conflict"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.MergePR(context.Background(), "owner", "repo", 12, "merge"); err == nil {
		t.Fatal("expected unmerged failure, got none")
	}
}
```

NOTE: `strings` import needed for `Repeat`.

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/githublive -run 'TestOpenPR|TestMerge' -count=1`

Expected: FAIL (`OpenPR`, `MergePR`, `NewPR`, `PR`, `Merge` undefined).

- [ ] **Step 3: Implement PR open/merge**

Branch names: `^[A-Za-z0-9._/-]{1,256}$` without `..`; title 1..256 runes; body ≤65536
runes. `OpenPR` posts `{base, head, title, body}`, requires `number > 0` and a 40-hex
head sha in response. `MergePR` rejects anything but `"merge"` before calling, puts
`{merge_method:"merge"}`, requires `merged == true` plus a 40-hex sha.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/githublive -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/githublive/pullrequest.go internal/githublive/pullrequest_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/githublive/pullrequest.go internal/githublive/pullrequest_test.go
git commit -m "feat(github): open and merge preserving prs"
```

### Task 3: Release, commit inspection, and retries

**Files:**
- Create: `internal/githublive/release.go`
- Test: `internal/githublive/release_test.go`

**Interfaces:**
- Consumes: `Client.do`/`Call` from Task 1.
- Produces: `(Client).CreateRelease(ctx, owner, repo, tag, commit, name, body)
  (Release, Call, error)`, `(Client).ListCommits(ctx, owner, repo, pr) ([]Commit, Call,
  error)`, `HasForeignCommits(commits, harnessAuthor) bool`, retry behavior for
  429/5xx (never other 4xx, never 501).

- [ ] **Step 1: Write the failing release test**

```go
package githublive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCreateRelease(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":7,"tag_name":"v1.2.3"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	rel, call, err := client.CreateRelease(context.Background(), "owner", "repo", "v1.2.3", strings.Repeat("f", 40), "v1.2.3", "Immutable release.")
	if err != nil {
		t.Fatal(err)
	}
	if rel.ID != 7 || rel.Tag != "v1.2.3" || call.StatusCode != 201 {
		t.Fatalf("unexpected release: %#v %#v", rel, call)
	}
	if _, _, err := client.CreateRelease(context.Background(), "owner", "repo", "1.2.3", strings.Repeat("f", 40), "x", "y"); err == nil {
		t.Fatal("expected tag rejection, got none")
	}
}

func TestListCommitsDetectsForeign(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"sha":"` + strings.Repeat("a", 40) + `","commit":{"author":{"name":"harness"}},"author":{"login":"harness-bot"}},{"sha":"` + strings.Repeat("b", 40) + `","commit":{"author":{"name":"contributor"}},"author":{"login":"octocat"}}]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	commits, _, err := client.ListCommits(context.Background(), "owner", "repo", 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("unexpected commits: %#v", commits)
	}
	if !HasForeignCommits(commits, "harness-bot") {
		t.Fatal("foreign commit missed")
	}
	if HasForeignCommits(commits[:1], "harness-bot") {
		t.Fatal("own commit flagged foreign")
	}
}

func TestRetryThenSucceeds(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"id":7,"tag_name":"v1.2.3"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.CreateRelease(context.Background(), "owner", "repo", "v1.2.3", strings.Repeat("f", 40), "v1.2.3", "x"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls.Load())
	}
}
```

NOTE: `strings` and `sync/atomic` imports. `Call.StatusCode` asserted 201 — the double
defaults to 200, so make the release double `w.WriteHeader(201)` when writing the file
(HTTP doubles default 200; assert what the double sends).

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/githublive -run 'TestCreateRelease|TestListCommits|TestRetryThenSucceeds' -count=1`

Expected: FAIL (`CreateRelease`, `ListCommits`, `HasForeignCommits` undefined).

- [ ] **Step 3: Implement releases, commits, and retries**

`CreateRelease` validates tag/commit (shared semver/sha patterns), posts
`{tag_name, target_commitish, name, body}`, requires `id > 0` and echoing tag.
`ListCommits` GETs `/pulls/{n}/commits`, parses `[{sha, author:{login}, commit.author.name}]`,
requires 40-hex shas, author = login if present else commit name. `HasForeignCommits`
compares each author to the harness author. Retry loop lives in `do`: 429 and
500/502/503/504 (not 501, not other 4xx), ≤3 attempts, `Retry-After` seconds capped
at 1s else 50ms steps, context-aware sleep.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/githublive -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/githublive/release.go internal/githublive/release_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/githublive/release.go internal/githublive/release_test.go
git commit -m "feat(github): release inspect commits and retry"
```

### Task 4: Prove the poller drives the daemon

**Files:**
- Create: `internal/daemon/github_test.go` (adapter + flow), extend `internal/daemon`
  with `IntakeFromIssues(owner, repo string, issues []githublive.Issue) []IntakeItem`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: `githublive.Poller` + daemon `Tick` from Tasks 1-3.
- Produces: executable evidence that a poller double feeds the daemon journal through
  submit→triage, plus the env-gated live test that skips by default.

- [ ] **Step 1: Add the flow test**

```go
package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/githublive"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestPollerDrivesDaemon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":78,"title":"Add health endpoint","body":"x","state":"open","user":{"login":"octocat"},"updated_at":"2026-09-23T00:00:00Z"}]`))
	}))
	defer server.Close()

	client, err := githublive.NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := githublive.NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	d, err := Open(testConfig(root), poller)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	didWork, err := d.Tick(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !didWork {
		t.Fatal("poller item not submitted")
	}
	item := mustLoad(t, d, "owner/repo#78")
	if item.State != workflow.StateTriage {
		t.Fatalf("poller item not triaged: %#v", item)
	}
}

func TestLiveGitHubIsGated(t *testing.T) {
	if os.Getenv("HARNESS_GITHUB_URL") == "" || os.Getenv("HARNESS_GITHUB_TOKEN") == "" {
		t.Skip("set HARNESS_GITHUB_URL and HARNESS_GITHUB_TOKEN to run against live GitHub")
	}
}
```

NOTE: `Open` takes `IntakeSource`; `*githublive.Poller` must satisfy it — but the
source interface method is `Poll(ctx, since, limit) ([]IntakeItem, error)` while the
poller returns `[]Issue`. The adapter bridges them: `pollerSource` in `daemon`
wrapping `*githublive.Poller` with `owner/repo` for `IntakeFromIssues`. But `daemon`
importing `githublive` while `githublive` must NOT import `daemon` (cycle!) — resolve:
`githublive.Issue` is standalone (no daemon import anywhere in githublive); the test
above passes `poller` directly only if signatures match, so instead the test wraps:
`Open(testConfig(root), &pollerAdapter{poller: poller})` where the tiny adapter lives
in the test file and calls `IntakeFromIssues`. Production wiring (cmd/harnessd flag
for GitHub polling) arrives with the human-surface increment — note it, do not build
half a wiring here.

- [ ] **Step 2: Run the flow test**

Run: `go test ./internal/daemon -run 'TestPollerDrivesDaemon|TestLiveGitHubIsGated' -count=1`

Expected: PASS (live test skips).

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean builds of all three binaries.

- [ ] **Step 4: Mark Increment 20 verified and commit**

In `docs/implementation-plan.md`, add Increment 20 to the v2 plan index and a
verification checklist. Do not mark it complete until the commands above pass on master.

```bash
git add internal/daemon/github_test.go internal/daemon/daemon.go docs/implementation-plan.md
git commit -m "test(github): verify poller drives daemon"
```
