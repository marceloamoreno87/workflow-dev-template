# Controlled Exposure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose Features through validated flag declarations with deterministic offline evaluation, staged rollouts gated by contributor acceptance, and scheduled flag removal that turns into tracked debt when overdue.

**Architecture:** `flags` owns flag declaration validation, deterministic evaluation with stable bucketing, rollout stage validation, acceptance decisions, disable semantics, and removal-deadline arithmetic as pure functions plus explicit clock parameters; it never touches the network or the filesystem. It may import nothing outside the standard library. All other modules stay untouched with no new imports. Provider wiring against GO Feature Flag, live targeting sync, and daemon-side rollout choreography belong to later increments and are explicitly out of scope: evaluation here mirrors provider semantics offline so targeting can be reasoned about without a server.

**Tech Stack:** Go 1.27.1 standard library only (`hash/fnv`, `strings`, `regexp`, `time`, table-driven tests)

**Spec:** `docs/workflow.md` (delivery sequence: targeted exposure, acceptance approve-or-changes, accepted progressive rollout then close plus flag removal within 14 days, rejected disable plus changes requested), `docs/architecture.md` (Module Delivery; repository shape `internal/flags`; no utils/common/manager), `docs/adr/0017-spike-go-feature-flag-first.md`, `CONTEXT.md` (Feature, Feature Flag, Exposure Strategy, Flag Debt, Acceptance Gate)

## Global Constraints

- Use Go 1.27.1.
- Standard library only; do not add dependencies.
- `internal/flags` imports nothing outside the standard library.
- Use the canonical terms from `CONTEXT.md` (Feature, Feature Flag, Exposure Strategy, Flag Debt, Acceptance Gate, Contributor); never write task or ticket for a Work Item.
- Flag keys match `[A-Za-z0-9_.-]{1,128}`; attribute names match `[A-Za-z0-9_]{1,64}`; percentages are closed-interval `[0,100]`.
- Evaluation is deterministic: the same flag key plus bucket key always yields the same variation (FNV-1a bucketing, no randomness, no clock).
- A disabled kill switch always evaluates off regardless of rules; an empty bucket key is rejected rather than bucketed ambiguously.
- Rollout stages are strictly increasing percentages ending exactly at 100; acceptance decisions belong to Contributors only (`operator` and `automation` are rejected as acceptors).
- A rejected Acceptance disables the Feature; acceptance never re-enables a killed flag (re-enable is a separate declaration change, out of scope).
- Removal deadline is completed-at plus 14 days; all time arrives via explicit parameters and the package never reads the clock.
- Do not add generic repository, provider, manager, service, utils, or common packages; new code lives in `internal/flags` only.

---

### Task 1: Validate flag declarations

**Files:**
- Create: `internal/flags/flag.go`
- Test: `internal/flags/flag_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `TargetRule struct`, `Flag struct`, `(Flag).Validate() error`, sentinel `ErrFlag`.

- [ ] **Step 1: Write the failing flag test**

```go
package flags

import (
	"testing"
)

func validFlag() Flag {
	return Flag{
		Key:     "checkout.redesign",
		Enabled: true,
		Rules: []TargetRule{
			{Name: "team", Attribute: "email", Values: []string{"@example.com"}, Percentage: 100, Variation: true},
		},
		DefaultOn:  false,
		Percentage: 10,
	}
}

func TestValidateFlag(t *testing.T) {
	t.Parallel()

	if err := validFlag().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadFlags(t *testing.T) {
	t.Parallel()

	mk := func(mut func(*Flag)) Flag {
		flag := validFlag()
		mut(&flag)
		return flag
	}
	for _, tc := range []struct {
		name string
		flag Flag
	}{
		{name: "empty key", flag: mk(func(f *Flag) { f.Key = "" })},
		{name: "bad key chars", flag: mk(func(f *Flag) { f.Key = "my flag!" })},
		{name: "too many rules", flag: mk(func(f *Flag) {
			f.Rules = make([]TargetRule, 17)
			for i := range f.Rules {
				f.Rules[i] = TargetRule{Name: "r", Attribute: "email", Values: []string{"a"}}
			}
		})},
		{name: "empty rule name", flag: mk(func(f *Flag) {
			f.Rules = []TargetRule{{Attribute: "email", Values: []string{"a"}}}
		})},
		{name: "bad attribute", flag: mk(func(f *Flag) {
			f.Rules = []TargetRule{{Name: "r", Attribute: "e-mail", Values: []string{"a"}}}
		})},
		{name: "no values", flag: mk(func(f *Flag) {
			f.Rules = []TargetRule{{Name: "r", Attribute: "email"}}
		})},
		{name: "bad percentage", flag: mk(func(f *Flag) {
			f.Rules = []TargetRule{{Name: "r", Attribute: "email", Values: []string{"a"}, Percentage: 101}}
		})},
		{name: "bad default percentage", flag: mk(func(f *Flag) { f.Percentage = -1 })},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.flag.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/flags -run 'TestValidateFlag|TestRejectBadFlags' -count=1`

Expected: FAIL because `Flag`, `TargetRule`, `Validate`, and `ErrFlag` are undefined.

- [ ] **Step 3: Implement flag validation**

```go
// internal/flags/flag.go
package flags

import (
	"errors"
	"fmt"
	"regexp"
)

var ErrFlag = errors.New("invalid feature flag")

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

var attributePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

type TargetRule struct {
	Name       string
	Attribute  string
	Values     []string
	Percentage float64
	Variation  bool
}

type Flag struct {
	Key        string
	Enabled    bool
	Rules      []TargetRule
	DefaultOn  bool
	Percentage float64
}

func validPercentage(p float64) bool {
	return p >= 0 && p <= 100
}

func (f Flag) Validate() error {
	if !keyPattern.MatchString(f.Key) {
		return fmt.Errorf("%w: key", ErrFlag)
	}
	if len(f.Rules) > 16 {
		return fmt.Errorf("%w: too many rules", ErrFlag)
	}
	for _, r := range f.Rules {
		if r.Name == "" || len([]rune(r.Name)) > 128 {
			return fmt.Errorf("%w: rule name", ErrFlag)
		}
		if !attributePattern.MatchString(r.Attribute) {
			return fmt.Errorf("%w: attribute %q", ErrFlag, r.Attribute)
		}
		if len(r.Values) == 0 || len(r.Values) > 64 {
			return fmt.Errorf("%w: rule %q needs 1..64 values", ErrFlag, r.Name)
		}
		for _, v := range r.Values {
			if n := len([]rune(v)); n == 0 || n > 256 {
				return fmt.Errorf("%w: value length %d", ErrFlag, n)
			}
		}
		if !validPercentage(r.Percentage) {
			return fmt.Errorf("%w: percentage %v", ErrFlag, r.Percentage)
		}
	}
	if !validPercentage(f.Percentage) {
		return fmt.Errorf("%w: default percentage %v", ErrFlag, f.Percentage)
	}
	return nil
}
```

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/flags -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/flags/flag.go internal/flags/flag_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/flags/flag.go internal/flags/flag_test.go
git commit -m "feat(flags): validate flag declarations"
```

### Task 2: Evaluate flags deterministically offline

**Files:**
- Create: `internal/flags/evaluate.go`
- Test: `internal/flags/evaluate_test.go`

**Interfaces:**
- Consumes: validated `Flag` from Task 1.
- Produces: `EvalContext struct`, `(Flag).Evaluate(ctx EvalContext) (bool, error)` with FNV-1a bucketing over `key + bucket key`.

- [ ] **Step 1: Write the failing evaluation test**

```go
package flags

import (
	"testing"
)

func TestEvaluateTargeting(t *testing.T) {
	t.Parallel()

	flag := validFlag()
	on, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "dev@example.com"}, BucketKey: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("matching rule at 100% should be on")
	}
	off, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "stranger@other.com"}, BucketKey: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	_ = off // percentage-dependent; asserted for stability below
	again, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "stranger@other.com"}, BucketKey: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if off != again {
		t.Fatal("evaluation is not deterministic")
	}
}

func TestDisabledIsAlwaysOff(t *testing.T) {
	t.Parallel()

	flag := validFlag()
	flag.Enabled = false
	on, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "dev@example.com"}, BucketKey: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("disabled flag evaluated on")
	}
}

func TestBucketBoundaries(t *testing.T) {
	t.Parallel()

	full := Flag{Key: "k", Enabled: true, Percentage: 100}
	for _, key := range []string{"a", "b", "user-999"} {
		on, err := full.Evaluate(EvalContext{BucketKey: key})
		if err != nil || !on {
			t.Fatalf("100%% should always be on: %v %v", on, err)
		}
	}
	none := Flag{Key: "k", Enabled: true, Percentage: 0}
	for _, key := range []string{"a", "b", "user-999"} {
		on, err := none.Evaluate(EvalContext{BucketKey: key})
		if err != nil || on {
			t.Fatalf("0%% should always be off: %v %v", on, err)
		}
	}
}

func TestRejectBadContexts(t *testing.T) {
	t.Parallel()

	flag := validFlag()
	if _, err := flag.Evaluate(EvalContext{BucketKey: ""}); err == nil {
		t.Fatal("expected empty bucket rejection, got none")
	}
	bad := validFlag()
	bad.Key = "bad key!"
	if _, err := bad.Evaluate(EvalContext{BucketKey: "u"}); err == nil {
		t.Fatal("expected invalid flag rejection, got none")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/flags -run 'TestEvaluate|TestDisabled|TestBucket|TestRejectBadContexts' -count=1`

Expected: FAIL because `Evaluate` and `EvalContext` are undefined.

- [ ] **Step 3: Implement deterministic evaluation**

```go
// internal/flags/evaluate.go
package flags

import (
	"fmt"
	"hash/fnv"
	"strings"
)

type EvalContext struct {
	Attributes map[string]string
	BucketKey  string
}

func bucket(key, bucketKey string, percentage float64) bool {
	h := fnv.New64a()
	h.Write([]byte(key + "\x00" + bucketKey))
	return h.Sum64()%10000 < uint64(percentage*100)
}

func ruleMatches(rule TargetRule, ctx EvalContext) bool {
	value, ok := ctx.Attributes[rule.Attribute]
	if !ok {
		return false
	}
	for _, candidate := range rule.Values {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func (f Flag) Evaluate(ctx EvalContext) (bool, error) {
	if err := f.Validate(); err != nil {
		return false, err
	}
	if ctx.BucketKey == "" || len([]rune(ctx.BucketKey)) > 256 {
		return false, fmt.Errorf("%w: bucket key required", ErrFlag)
	}
	if len(ctx.Attributes) > 32 {
		return false, fmt.Errorf("%w: too many attributes", ErrFlag)
	}
	if !f.Enabled {
		return false, nil
	}
	for _, rule := range f.Rules {
		if ruleMatches(rule, ctx) {
			if rule.Percentage >= 100 {
				return rule.Variation, nil
			}
			if !bucket(f.Key+":"+rule.Name, ctx.BucketKey, rule.Percentage) {
				return !rule.Variation, nil
			}
			return rule.Variation, nil
		}
	}
	if f.Percentage >= 100 {
		return true, nil
	}
	if f.Percentage <= 0 && !f.DefaultOn {
		return false, nil
	}
	if bucket(f.Key, ctx.BucketKey, f.Percentage) {
		return true, nil
	}
	return f.DefaultOn, nil
}
```

NOTE on matching semantics: rule values match by substring (`strings.Contains`) so entries like `@example.com` match full emails without requiring exact-equality configuration. Percentage inside a matched rule re-buckets on `key:rule`; outside it falls through to the flag percentage, then the default. Keep these semantics; the tests above pin them.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/flags -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/flags/evaluate.go internal/flags/evaluate_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/flags/evaluate.go internal/flags/evaluate_test.go
git commit -m "feat(flags): evaluate flags deterministically"
```

### Task 3: Stage rollouts, decide acceptance, track debt

**Files:**
- Create: `internal/flags/rollout.go`
- Test: `internal/flags/rollout_test.go`

**Interfaces:**
- Consumes: nothing beyond stdlib.
- Produces: `RolloutStage struct`, `ValidateRollout(stages []RolloutStage) error`, `Acceptance struct`, `DecideAcceptance(a Acceptance) (advance, disable bool, err error)`, `RemovalDeadline(completedAt time.Time) time.Time`, `DebtOverdue(completedAt, now time.Time) bool`.

- [ ] **Step 1: Write the failing rollout test**

```go
package flags

import (
	"testing"
	"time"
)

func TestValidateRollout(t *testing.T) {
	t.Parallel()

	stages := []RolloutStage{{Percentage: 1}, {Percentage: 10}, {Percentage: 50}, {Percentage: 100}}
	if err := ValidateRollout(stages); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]RolloutStage{
		nil,
		{{Percentage: 100}, {Percentage: 50}},
		{{Percentage: 10}, {Percentage: 10}},
		{{Percentage: 50}},
		{{Percentage: 0}},
		{{Percentage: 101}},
	} {
		if err := ValidateRollout(bad); err == nil {
			t.Fatalf("expected rejection for %v, got none", bad)
		}
	}
}

func TestAcceptanceDecides(t *testing.T) {
	t.Parallel()

	advance, disable, err := DecideAcceptance(Acceptance{WorkItem: "owner/repo#123", Decision: "accept", By: "actor/client-a", ByKind: "contributor"})
	if err != nil || !advance || disable {
		t.Fatalf("accept should advance: %v %v %v", advance, disable, err)
	}
	advance, disable, err = DecideAcceptance(Acceptance{WorkItem: "owner/repo#123", Decision: "reject", By: "actor/client-a", ByKind: "contributor"})
	if err != nil || advance || !disable {
		t.Fatalf("reject should disable: %v %v %v", advance, disable, err)
	}
	for _, bad := range []Acceptance{
		{WorkItem: "", Decision: "accept", By: "actor/client-a", ByKind: "contributor"},
		{WorkItem: "owner/repo#123", Decision: "maybe", By: "actor/client-a", ByKind: "contributor"},
		{WorkItem: "owner/repo#123", Decision: "accept", By: "actor/operator", ByKind: "operator"},
		{WorkItem: "owner/repo#123", Decision: "accept", By: "actor/automation", ByKind: "automation"},
	} {
		if _, _, err := DecideAcceptance(bad); err == nil {
			t.Fatalf("expected rejection for %#v, got none", bad)
		}
	}
}

func TestDebtDeadline(t *testing.T) {
	t.Parallel()

	done := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if got := RemovalDeadline(done); !got.Equal(done.Add(14 * 24 * time.Hour)) {
		t.Fatalf("unexpected deadline: %v", got)
	}
	if DebtOverdue(done, done.Add(13*24*time.Hour)) {
		t.Fatal("not overdue before 14 days")
	}
	if !DebtOverdue(done, done.Add(15*24*time.Hour)) {
		t.Fatal("overdue after 14 days")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/flags -run 'TestValidateRollout|TestAcceptanceDecides|TestDebtDeadline' -count=1`

Expected: FAIL because `RolloutStage`, `ValidateRollout`, `Acceptance`, `DecideAcceptance`, `RemovalDeadline`, and `DebtOverdue` are undefined.

- [ ] **Step 3: Implement rollout, acceptance, and debt**

```go
// internal/flags/rollout.go
package flags

import (
	"fmt"
	"time"
)

type RolloutStage struct {
	Percentage float64
}

func ValidateRollout(stages []RolloutStage) error {
	if len(stages) == 0 {
		return fmt.Errorf("%w: rollout needs stages", ErrFlag)
	}
	previous := -1.0
	for _, stage := range stages {
		if !(stage.Percentage > 0 && stage.Percentage <= 100) {
			return fmt.Errorf("%w: stage percentage %v", ErrFlag, stage.Percentage)
		}
		if stage.Percentage <= previous {
			return fmt.Errorf("%w: stages must strictly increase", ErrFlag)
		}
		previous = stage.Percentage
	}
	if previous != 100 {
		return fmt.Errorf("%w: rollout must end at 100", ErrFlag)
	}
	return nil
}

type Acceptance struct {
	WorkItem string
	Decision string
	By       string
	ByKind   string
}

func DecideAcceptance(a Acceptance) (advance, disable bool, err error) {
	if a.WorkItem == "" || a.By == "" {
		return false, false, fmt.Errorf("%w: work item and acceptor required", ErrFlag)
	}
	if a.ByKind != "contributor" {
		return false, false, fmt.Errorf("%w: acceptance belongs to contributors", ErrFlag)
	}
	switch a.Decision {
	case "accept":
		return true, false, nil
	case "reject":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("%w: decision %q", ErrFlag, a.Decision)
	}
}

func RemovalDeadline(completedAt time.Time) time.Time {
	return completedAt.Add(14 * 24 * time.Hour)
}

func DebtOverdue(completedAt, now time.Time) bool {
	return !now.Before(RemovalDeadline(completedAt))
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/flags -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/flags/rollout.go internal/flags/rollout_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/flags/rollout.go internal/flags/rollout_test.go
git commit -m "feat(flags): stage rollouts and track debt"
```

### Task 4: Prove the targeted-to-cleanup flow end-to-end

**Files:**
- Create: `internal/flags/flow_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete flag lifecycle from Tasks 1-3.
- Produces: executable evidence of targeted exposure → acceptance → staged rollout → completion → debt, plus the reject-disables branch.

- [ ] **Step 1: Add the flow test**

```go
package flags

import (
	"testing"
	"time"
)

func TestExposureFlow(t *testing.T) {
	t.Parallel()

	flag := Flag{
		Key:     "checkout.redesign",
		Enabled: true,
		Rules: []TargetRule{
			{Name: "team", Attribute: "email", Values: []string{"@example.com"}, Percentage: 100, Variation: true},
		},
		Percentage: 1,
	}
	if err := flag.Validate(); err != nil {
		t.Fatal(err)
	}
	team, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "dev@example.com"}, BucketKey: "user-1"})
	if err != nil || !team {
		t.Fatalf("team should see the feature: %v %v", team, err)
	}
	stages := []RolloutStage{{Percentage: 1}, {Percentage: 10}, {Percentage: 50}, {Percentage: 100}}
	if err := ValidateRollout(stages); err != nil {
		t.Fatal(err)
	}
	advance, disable, err := DecideAcceptance(Acceptance{WorkItem: "owner/repo#123", Decision: "accept", By: "actor/client-a", ByKind: "contributor"})
	if err != nil || !advance || disable {
		t.Fatalf("acceptance should advance: %v %v %v", advance, disable, err)
	}
	completedAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if DebtOverdue(completedAt, completedAt.Add(13*24*time.Hour)) {
		t.Fatal("flag removed in time is not debt")
	}
	if !DebtOverdue(completedAt, completedAt.Add(15*24*time.Hour)) {
		t.Fatal("flag surviving past removal is debt")
	}
}

func TestRejectedAcceptanceDisables(t *testing.T) {
	t.Parallel()

	flag := validFlag()
	_, disable, err := DecideAcceptance(Acceptance{WorkItem: "owner/repo#123", Decision: "reject", By: "actor/client-a", ByKind: "contributor"})
	if err != nil || !disable {
		t.Fatalf("reject should disable: %v %v", disable, err)
	}
	flag.Enabled = false
	on, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "dev@example.com"}, BucketKey: "user-1"})
	if err != nil || on {
		t.Fatalf("disabled flag must stay off: %v %v", on, err)
	}
}
```

- [ ] **Step 2: Run the flow test**

Run: `go test ./internal/flags -run 'TestExposureFlow|TestRejectedAcceptanceDisables' -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean build of `cmd/harness`.

- [ ] **Step 4: Mark Increment 11 verified and commit**

In `docs/implementation-plan.md`, add Increment 11 to the plan index and a verification checklist below Increment 10. Do not mark it complete until the commands above pass on master.

```bash
git add internal/flags/flow_test.go docs/implementation-plan.md
git commit -m "test(flags): verify exposure to cleanup flow"
```
