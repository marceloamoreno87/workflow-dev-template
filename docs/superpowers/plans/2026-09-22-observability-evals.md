# Observability and Evals Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route every Agent Thread through the documented model baseline, attribute token usage and cost per Work Item and model, capture redacted spans locally, and lock the routing table behind an executable eval baseline.

**Architecture:** `telemetry` owns the routing table, usage records with cost attribution, a bounded span recorder with secret redaction, JSON export, and the eval runner as pure functions plus explicit clock parameters; it never touches the network and never reads the clock. It may import nothing outside the standard library. All other modules stay untouched with no new imports except a one-line widening of the `codex` effort allowlist (see Task 1). OTLP export, SQLite summaries, log rotation, and promotion metrics belong to later increments and are explicitly out of scope.

**Tech Stack:** Go 1.27.1 standard library only (`crypto/rand`, `encoding/json`, `strings`, `time`, table-driven tests)

**Spec:** `docs/model-routing.md` (the 11-row routing table with Luna/Terra/Sol and low→xhigh efforts; routing changes only after representative evals; model/effort/tokens/duration/cost are telemetry dimensions), `docs/architecture.md` (Module boundaries; no utils/common/manager), `docs/adr/0010-capture-local-telemetry-with-opentelemetry.md`, `docs/adr/0015-route-codex-models-by-task.md`, `docs/contracts.md` (events carry redacted telemetry references)

## Global Constraints

- Use Go 1.27.1.
- Standard library only; do not add dependencies.
- `internal/telemetry` imports nothing outside the standard library.
- Use the canonical terms from `CONTEXT.md` (Agent Role, Agent Thread, Work Item, Spec, Quality Profile, Data Classification, Observation); never write agent or memory for those meanings.
- The routing table pins the exact models and efforts from `docs/model-routing.md`, including `xhigh` for persistent-failure escalation; unknown task kinds are rejected rather than defaulted so new work is routed deliberately.
- Data Classification `confidential` and `restricted` never route to any model (same gate as the codex runtime).
- Escalation never resets budgets: routing carries no budget state and the eval baseline asserts Luna→Terra→Sol ordering only through the table, not through runtime counters.
- Span attribute keys are split on non-alphanumeric parts and any part matching `token`, `secret`, `passwd`, `password`, `credentials`, `credential`, `private`, `key`, or `auth` stores `[redacted]` (fail-safe: `api_key` redacts, `monkey` does not); the raw values never reach the recorder or the export.
- The recorder is bounded (4096 spans, reject when full); export is newline-delimited JSON, one span per line, with no secret-bearing content by construction.
- All time arrives via explicit parameters; IDs generate from `crypto/rand` and tests assert shape, never values.
- Do not add generic repository, provider, manager, service, utils, or common packages; new code lives in `internal/telemetry` only (plus the one-line codex widening).

---

### Task 1: Route threads through the documented baseline

**Files:**
- Modify: `internal/codex/thread.go`, `internal/codex/thread_test.go` (allow `xhigh`)
- Create: `internal/telemetry/route.go`
- Test: `internal/telemetry/route_test.go`

**Interfaces:**
- Consumes: `docs/model-routing.md` table.
- Produces: `Route{Model, Effort}`, `Select(taskKind, classification string) (Route, error)`, sentinel `ErrTelemetry`, task-kind allowlist of the 11 documented rows.

- [ ] **Step 1: Widen the codex effort allowlist with its test**

In `internal/codex/thread.go`, change the effort switch to `case "low", "medium", "high", "xhigh":`. In `internal/codex/thread_test.go`, extend the bad-effort coverage so `xhigh` is accepted (add a valid-spec variant asserting `Effort: "xhigh"` validates). Run `go test ./internal/codex -count=1` (expect PASS) and commit separately:

```bash
git add internal/codex/thread.go internal/codex/thread_test.go
git commit -m "feat(codex): allow xhigh reasoning effort"
```

Rationale to record in the commit body: the routing canon pins `xhigh` for persistent-failure escalation, so the runtime must express it; whether a given codex binary accepts the value surfaces as a clear runtime error, never a silent downgrade.

- [ ] **Step 2: Write the failing routing test**

```go
package telemetry

import (
	"testing"
)

func TestSelectBaseline(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		kind   string
		model  string
		effort string
	}{
		{kind: "triage", model: "gpt-5.6-luna", effort: "low"},
		{kind: "summarize", model: "gpt-5.6-luna", effort: "low"},
		{kind: "index", model: "gpt-5.6-luna", effort: "low"},
		{kind: "mechanical", model: "gpt-5.6-luna", effort: "low"},
		{kind: "explore", model: "gpt-5.6-terra", effort: "low"},
		{kind: "spec", model: "gpt-5.6-terra", effort: "medium"},
		{kind: "implement", model: "gpt-5.6-terra", effort: "medium"},
		{kind: "fix", model: "gpt-5.6-terra", effort: "medium"},
		{kind: "review", model: "gpt-5.6-terra", effort: "high"},
		{kind: "architecture", model: "gpt-5.6-sol", effort: "high"},
		{kind: "cross-cutting", model: "gpt-5.6-sol", effort: "medium"},
		{kind: "security-review", model: "gpt-5.6-sol", effort: "high"},
		{kind: "critical-review", model: "gpt-5.6-sol", effort: "high"},
		{kind: "escalate", model: "gpt-5.6-sol", effort: "xhigh"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			route, err := Select(tc.kind, "internal")
			if err != nil {
				t.Fatal(err)
			}
			if route.Model != tc.model || route.Effort != tc.effort {
				t.Fatalf("got %#v", route)
			}
		})
	}
}

func TestSelectRejects(t *testing.T) {
	t.Parallel()

	if _, err := Select("teleport", "internal"); err == nil {
		t.Fatal("expected unknown-kind rejection, got none")
	}
	for _, classification := range []string{"confidential", "restricted", "bogus"} {
		if _, err := Select("triage", classification); err == nil {
			t.Fatalf("expected classification rejection for %q, got none", classification)
		}
	}
	if _, err := Select("triage", "public"); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 3: Run the test and verify it fails**

Run: `go test ./internal/telemetry -run 'TestSelect' -count=1`

Expected: FAIL because `Select`, `Route`, and `ErrTelemetry` are undefined.

- [ ] **Step 4: Implement the routing table**

```go
// internal/telemetry/route.go
package telemetry

import (
	"errors"
	"fmt"
)

var ErrTelemetry = errors.New("invalid telemetry input")

type Route struct {
	Model  string
	Effort string
}

var baseline = map[string]Route{
	"triage":           {Model: "gpt-5.6-luna", Effort: "low"},
	"summarize":        {Model: "gpt-5.6-luna", Effort: "low"},
	"index":            {Model: "gpt-5.6-luna", Effort: "low"},
	"mechanical":       {Model: "gpt-5.6-luna", Effort: "low"},
	"explore":          {Model: "gpt-5.6-terra", Effort: "low"},
	"spec":             {Model: "gpt-5.6-terra", Effort: "medium"},
	"implement":        {Model: "gpt-5.6-terra", Effort: "medium"},
	"fix":              {Model: "gpt-5.6-terra", Effort: "medium"},
	"review":           {Model: "gpt-5.6-terra", Effort: "high"},
	"architecture":     {Model: "gpt-5.6-sol", Effort: "high"},
	"cross-cutting":    {Model: "gpt-5.6-sol", Effort: "medium"},
	"security-review":  {Model: "gpt-5.6-sol", Effort: "high"},
	"critical-review":  {Model: "gpt-5.6-sol", Effort: "high"},
	"escalate":         {Model: "gpt-5.6-sol", Effort: "xhigh"},
}

func Select(taskKind, classification string) (Route, error) {
	switch classification {
	case "public", "internal":
	default:
		return Route{}, fmt.Errorf("%w: classification %q never routes to a model", ErrTelemetry, classification)
	}
	route, ok := baseline[taskKind]
	if !ok {
		return Route{}, fmt.Errorf("%w: unknown task kind %q", ErrTelemetry, taskKind)
	}
	return route, nil
}
```

- [ ] **Step 5: Run the package test**

Run: `go test ./internal/telemetry -count=1`

Expected: PASS.

- [ ] **Step 6: Format and commit**

Run: `gofmt -w internal/telemetry/route.go internal/telemetry/route_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/telemetry/route.go internal/telemetry/route_test.go
git commit -m "feat(telemetry): route threads through the baseline"
```

### Task 2: Attribute usage and cost

**Files:**
- Create: `internal/telemetry/usage.go`
- Test: `internal/telemetry/usage_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `UsageRecord struct`, `(UsageRecord).Validate() error`, `Attribution struct`, `Attribute(records []UsageRecord) (Attribution, error)`.

- [ ] **Step 1: Write the failing usage test**

```go
package telemetry

import (
	"testing"
	"time"
)

func TestAttributeCost(t *testing.T) {
	t.Parallel()

	records := []UsageRecord{
		{WorkItem: "owner/repo#1", Model: "gpt-5.6-terra", InputTokens: 1000, OutputTokens: 500, CostUSD: 0.05, Duration: time.Minute},
		{WorkItem: "owner/repo#1", Model: "gpt-5.6-luna", InputTokens: 2000, OutputTokens: 100, CostUSD: 0.01, Duration: time.Minute},
		{WorkItem: "owner/repo#2", Model: "gpt-5.6-terra", InputTokens: 500, OutputTokens: 500, CostUSD: 0.03, Duration: time.Minute},
	}
	attribution, err := Attribute(records)
	if err != nil {
		t.Fatal(err)
	}
	if attribution.ByWorkItem["owner/repo#1"].CostUSD != 0.06 {
		t.Fatalf("work item cost wrong: %#v", attribution.ByWorkItem["owner/repo#1"])
	}
	if attribution.ByModel["gpt-5.6-terra"].OutputTokens != 1000 {
		t.Fatalf("model tokens wrong: %#v", attribution.ByModel["gpt-5.6-terra"])
	}
	if attribution.Total.CostUSD != 0.09 || attribution.Total.InputTokens != 3500 {
		t.Fatalf("totals wrong: %#v", attribution.Total)
	}
}

func TestRejectBadUsage(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		record UsageRecord
	}{
		{name: "empty work item", record: UsageRecord{Model: "m", CostUSD: 0.01}},
		{name: "empty model", record: UsageRecord{WorkItem: "w", CostUSD: 0.01}},
		{name: "negative tokens", record: UsageRecord{WorkItem: "w", Model: "m", InputTokens: -1}},
		{name: "negative cost", record: UsageRecord{WorkItem: "w", Model: "m", CostUSD: -0.01}},
		{name: "negative duration", record: UsageRecord{WorkItem: "w", Model: "m", Duration: -time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Attribute([]UsageRecord{tc.record}); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
	if _, err := Attribute(nil); err == nil {
		t.Fatal("expected empty rejection, got none")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/telemetry -run 'TestAttribute|TestRejectBadUsage' -count=1`

Expected: FAIL because `UsageRecord`, `Attribution`, and `Attribute` are undefined.

- [ ] **Step 3: Implement usage attribution**

```go
// internal/telemetry/usage.go
package telemetry

import (
	"fmt"
	"time"
)

type UsageRecord struct {
	WorkItem     string
	Model        string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	Duration     time.Duration
}

type UsageTotal struct {
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	Duration     time.Duration
}

type Attribution struct {
	ByWorkItem map[string]UsageTotal
	ByModel    map[string]UsageTotal
	Total      UsageTotal
}

func (r UsageRecord) Validate() error {
	if r.WorkItem == "" {
		return fmt.Errorf("%w: work item required", ErrTelemetry)
	}
	if r.Model == "" {
		return fmt.Errorf("%w: model required", ErrTelemetry)
	}
	if r.InputTokens < 0 || r.OutputTokens < 0 {
		return fmt.Errorf("%w: negative tokens", ErrTelemetry)
	}
	if r.CostUSD < 0 {
		return fmt.Errorf("%w: negative cost", ErrTelemetry)
	}
	if r.Duration < 0 {
		return fmt.Errorf("%w: negative duration", ErrTelemetry)
	}
	return nil
}

func (t *UsageTotal) add(r UsageRecord) {
	t.InputTokens += r.InputTokens
	t.OutputTokens += r.OutputTokens
	t.CostUSD += r.CostUSD
	t.Duration += r.Duration
}

func Attribute(records []UsageRecord) (Attribution, error) {
	if len(records) == 0 {
		return Attribution{}, fmt.Errorf("%w: no usage records", ErrTelemetry)
	}
	out := Attribution{ByWorkItem: map[string]UsageTotal{}, ByModel: map[string]UsageTotal{}}
	for _, r := range records {
		if err := r.Validate(); err != nil {
			return Attribution{}, err
		}
		work := out.ByWorkItem[r.WorkItem]
		work.add(r)
		out.ByWorkItem[r.WorkItem] = work
		model := out.ByModel[r.Model]
		model.add(r)
		out.ByModel[r.Model] = model
		out.Total.add(r)
	}
	return out, nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/telemetry -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/telemetry/usage.go internal/telemetry/usage_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/telemetry/usage.go internal/telemetry/usage_test.go
git commit -m "feat(telemetry): attribute usage and cost"
```

### Task 3: Record redacted spans with bounded export

**Files:**
- Create: `internal/telemetry/span.go`
- Test: `internal/telemetry/span_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `Span struct`, `NewSpan(name string, startedAt time.Time) (Span, error)`, `Recorder` with `Record`/`Export`, 4096-span bound, substring redaction.

- [ ] **Step 1: Write the failing span test**

```go
package telemetry

import (
	"strings"
	"testing"
	"time"
)

func TestRecordAndExport(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()
	span, err := NewSpan("implement", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	span.WorkItem = "owner/repo#1"
	span.Finish(time.Now().Add(time.Minute))
	span.Attrs = map[string]string{"model": "gpt-5.6-terra", "github_token": "s3cr3t"}
	if err := rec.Record(span); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := rec.Export(&b); err != nil {
		t.Fatal(err)
	}
	exported := b.String()
	if strings.Contains(exported, "s3cr3t") {
		t.Fatalf("secret reached export:\n%s", exported)
	}
	if !strings.Contains(exported, "[redacted]") || !strings.Contains(exported, span.SpanID) {
		t.Fatalf("redaction or identity missing:\n%s", exported)
	}
}

func TestSpanIDsAreShaped(t *testing.T) {
	t.Parallel()

	span, err := NewSpan("review", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(span.TraceID) != 32 || len(span.SpanID) != 16 {
		t.Fatalf("bad id shapes: %#v", span)
	}
	for _, c := range span.TraceID + span.SpanID {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			t.Fatalf("non-hex id: %#v", span)
		}
	}
}

func TestRejectBadSpans(t *testing.T) {
	t.Parallel()

	now := time.Now()
	if _, err := NewSpan("", now); err == nil {
		t.Fatal("expected empty-name rejection, got none")
	}
	span, err := NewSpan("x", now)
	if err != nil {
		t.Fatal(err)
	}
	span.Finish(now.Add(-time.Second))
	if err := NewRecorder().Record(span); err == nil {
		t.Fatal("expected inverted-window rejection, got none")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/telemetry -run 'TestRecordAndExport|TestSpanIDs|TestRejectBadSpans' -count=1`

Expected: FAIL because `Span`, `NewSpan`, and `NewRecorder` are undefined.

- [ ] **Step 3: Implement spans with redaction**

```go
// internal/telemetry/span.go
package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const maxSpans = 4096

var secretParts = map[string]bool{
	"token": true, "secret": true, "passwd": true, "password": true,
	"credentials": true, "credential": true, "private": true, "key": true, "auth": true,
}

func secretKey(key string) bool {
	var part strings.Builder
	flush := func() bool {
		hit := secretParts[strings.ToLower(part.String())]
		part.Reset()
		return hit
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			part.WriteRune(r)
			continue
		}
		if flush() {
			return true
		}
	}
	return flush()
}

type Span struct {
	TraceID  string
	SpanID   string
	ParentID string
	Name     string
	WorkItem string
	StartedAt time.Time
	FinishedAt time.Time
	Attrs    map[string]string
}

func hexID(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func NewSpan(name string, startedAt time.Time) (Span, error) {
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 256 {
		return Span{}, fmt.Errorf("%w: span name", ErrTelemetry)
	}
	trace, err := hexID(16)
	if err != nil {
		return Span{}, err
	}
	id, err := hexID(8)
	if err != nil {
		return Span{}, err
	}
	return Span{TraceID: trace, SpanID: id, Name: name, StartedAt: startedAt}, nil
}

func (s *Span) Finish(finishedAt time.Time) {
	s.FinishedAt = finishedAt
}

func redact(attrs map[string]string) map[string]string {
	if attrs == nil {
		return nil
	}
	out := make(map[string]string, len(attrs))
	for key, value := range attrs {
		if secretKey(key) {
			out[key] = "[redacted]"
			continue
		}
		out[key] = value
	}
	return out
}

func (s Span) validate() error {
	if len(s.TraceID) != 32 || len(s.SpanID) != 16 {
		return fmt.Errorf("%w: span identity", ErrTelemetry)
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("%w: span name", ErrTelemetry)
	}
	if s.FinishedAt.Before(s.StartedAt) {
		return fmt.Errorf("%w: span window inverted", ErrTelemetry)
	}
	if len(s.Attrs) > 32 {
		return fmt.Errorf("%w: too many attributes", ErrTelemetry)
	}
	return nil
}

type Recorder struct {
	spans []Span
}

func NewRecorder() *Recorder {
	return &Recorder{}
}

func (r *Recorder) Record(span Span) error {
	if err := span.validate(); err != nil {
		return err
	}
	if len(r.spans) >= maxSpans {
		return fmt.Errorf("%w: recorder full", ErrTelemetry)
	}
	span.Attrs = redact(span.Attrs)
	r.spans = append(r.spans, span)
	return nil
}

func (r *Recorder) Export(w io.Writer) error {
	for _, span := range r.spans {
		raw, err := json.Marshal(map[string]any{
			"traceId":  span.TraceID,
			"spanId":   span.SpanID,
			"parentId": span.ParentID,
			"name":     span.Name,
			"workItem": span.WorkItem,
			"started":  span.StartedAt.UTC().Format(time.RFC3339Nano),
			"finished": span.FinishedAt.UTC().Format(time.RFC3339Nano),
			"attrs":    span.Attrs,
		})
		if err != nil {
			return err
		}
		if _, err := w.Write(append(raw, '\n')); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/telemetry -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/telemetry/span.go internal/telemetry/span_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/telemetry/span.go internal/telemetry/span_test.go
git commit -m "feat(telemetry): record redacted spans"
```

### Task 4: Lock the routing baseline behind evals

**Files:**
- Create: `internal/telemetry/eval_test.go` (eval runner lives in `route.go` or a small `eval.go`; prefer `eval.go` with `EvalCase` + `RunEval`)
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: `Select` from Task 1.
- Produces: executable proof that all 14 documented routing rows resolve and that the flow route → usage → spans → export holds together.

- [ ] **Step 1: Add the eval test**

```go
package telemetry

import (
	"strings"
	"testing"
	"time"
)

type EvalCase struct {
	TaskKind  string
	Model     string
	Effort    string
}

func runEval(cases []EvalCase) []string {
	var failures []string
	for _, c := range cases {
		route, err := Select(c.TaskKind, "internal")
		if err != nil || route.Model != c.Model || route.Effort != c.Effort {
			failures = append(failures, c.TaskKind)
		}
	}
	return failures
}

func TestRoutingBaselineEval(t *testing.T) {
	t.Parallel()

	cases := []EvalCase{
		{TaskKind: "triage", Model: "gpt-5.6-luna", Effort: "low"},
		{TaskKind: "summarize", Model: "gpt-5.6-luna", Effort: "low"},
		{TaskKind: "index", Model: "gpt-5.6-luna", Effort: "low"},
		{TaskKind: "mechanical", Model: "gpt-5.6-luna", Effort: "low"},
		{TaskKind: "explore", Model: "gpt-5.6-terra", Effort: "low"},
		{TaskKind: "spec", Model: "gpt-5.6-terra", Effort: "medium"},
		{TaskKind: "implement", Model: "gpt-5.6-terra", Effort: "medium"},
		{TaskKind: "fix", Model: "gpt-5.6-terra", Effort: "medium"},
		{TaskKind: "review", Model: "gpt-5.6-terra", Effort: "high"},
		{TaskKind: "architecture", Model: "gpt-5.6-sol", Effort: "high"},
		{TaskKind: "cross-cutting", Model: "gpt-5.6-sol", Effort: "medium"},
		{TaskKind: "security-review", Model: "gpt-5.6-sol", Effort: "high"},
		{TaskKind: "critical-review", Model: "gpt-5.6-sol", Effort: "high"},
		{TaskKind: "escalate", Model: "gpt-5.6-sol", Effort: "xhigh"},
	}
	if failures := runEval(cases); len(failures) > 0 {
		t.Fatalf("baseline drift: %v", failures)
	}
}

func TestObservabilityFlow(t *testing.T) {
	t.Parallel()

	route, err := Select("implement", "internal")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	span, err := NewSpan("implement", now)
	if err != nil {
		t.Fatal(err)
	}
	span.WorkItem = "owner/repo#1"
	span.Attrs = map[string]string{"model": route.Model, "effort": route.Effort, "api_key": "s3cr3t"}
	span.Finish(now.Add(2 * time.Minute))
	rec := NewRecorder()
	if err := rec.Record(span); err != nil {
		t.Fatal(err)
	}
	attribution, err := Attribute([]UsageRecord{{
		WorkItem: "owner/repo#1", Model: route.Model,
		InputTokens: 1200, OutputTokens: 300, CostUSD: 0.04, Duration: 2 * time.Minute,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if attribution.ByModel[route.Model].CostUSD != 0.04 {
		t.Fatalf("cost lost: %#v", attribution)
	}
	var b strings.Builder
	if err := rec.Export(&b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "s3cr3t") {
		t.Fatalf("secret reached export:\n%s", b.String())
	}
}
```

- [ ] **Step 2: Run the eval test**

Run: `go test ./internal/telemetry -run 'TestRoutingBaselineEval|TestObservabilityFlow' -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean build of `cmd/harness`.

- [ ] **Step 4: Mark Increment 15 verified and commit**

In `docs/implementation-plan.md`, add Increment 15 to the plan index and a verification checklist below Increment 14. Do not mark it complete until the commands above pass on master.

```bash
git add internal/telemetry/eval_test.go docs/implementation-plan.md
git commit -m "test(telemetry): lock routing baseline behind evals"
```
