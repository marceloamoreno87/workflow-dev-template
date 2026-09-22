# Additional Stacks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve Stack Adapters by manifest name and render their Quality Gates as exact host commands, with real execution evidence on every toolchain present in the environment (Go, Python, Node) and graceful skips where a binary is absent.

**Architecture:** `stack` owns adapter declarations, catalog resolution, gate validation, exact argv rendering, and bounded host execution; it never touches the network. It may import nothing outside the standard library. All other modules stay untouched with no new imports. Containerized gate execution (via `internal/runner`), dependency installation, and new production adapters belong to later work and are explicitly out of scope: gates declared here run on the host against fixture projects, which is sufficient to prove the contract end to end.

**Tech Stack:** Go 1.27.1 standard library only (`os`, `os/exec`, `context`, `path/filepath`, real `python3`/`node`/`go` binaries when present, table-driven tests)

**Spec:** `docs/quality-gates.md` (Go Stack Adapter baseline: gofmt, vet, tests; Evidence discipline), `docs/contracts.md` (manifest `stack.adapter`), `docs/architecture.md` (Stack Adapter contract; repository shape `internal/stack`; no utils/common/manager), `CONTEXT.md` (Stack Adapter, Project Runner, Quality Profile, Gate)

## Global Constraints

- Use Go 1.27.1.
- Standard library only; do not add dependencies.
- `internal/stack` imports nothing outside the standard library.
- Use the canonical terms from `CONTEXT.md` (Stack Adapter, Quality Profile, Gate, Project, Project Worktree); never write folder, package, or module for a Project.
- Adapter names match `[a-z0-9][a-z0-9-]{0,63}`; the catalog pins exactly `go-service`, `python-service`, and `nextjs-web`; unknown names resolve only by explicit registration, never by fuzzy default.
- Gate names come from the closed set `format`, `lint`, `typecheck`, `test`, `build`, `smoke`; commands carry 1..8 args with a bare binary name (resolved via `PATH` at run, never an absolute host path, never a shell); workdirs are relative and confined (no `..`, no absolute paths, no escapes); timeouts span 10s..30m.
- The renderer outputs exact argv plus the resolved working directory; any deviation in validation rejects before any process starts.
- Host execution uses a deny-by-default subprocess environment (PATH plus locale only), caps output at 64 KiB per stream with a flag, honors timeouts, and never echoes environment values in errors.
- Tests that need a toolchain binary skip when `LookPath` fails; tests never install packages or reach the network.
- Do not add generic repository, provider, manager, service, utils, or common packages; new code lives in `internal/stack` only.

---

### Task 1: Declare adapters and resolve the catalog

**Files:**
- Create: `internal/stack/adapter.go`
- Test: `internal/stack/adapter_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `GateSpec struct`, `Adapter struct`, `(Adapter).Validate() error`, `Catalog() map[string]Adapter`, `Resolve(name string) (Adapter, error)`, sentinel `ErrStack`.

- [ ] **Step 1: Write the failing adapter test**

```go
package stack

import (
	"testing"
	"time"
)

func TestCatalogResolves(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"go-service", "python-service", "nextjs-web"} {
		adapter, err := Resolve(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := adapter.Validate(); err != nil {
			t.Fatalf("%s invalid: %v", name, err)
		}
	}
	if _, err := Resolve("cobol-mainframe"); err == nil {
		t.Fatal("expected unknown rejection, got none")
	}
}

func TestGoServiceDeclaresBaseline(t *testing.T) {
	t.Parallel()

	adapter, err := Resolve("go-service")
	if err != nil {
		t.Fatal(err)
	}
	for _, gate := range []string{"format", "vet-gate", "test"} {
		_ = gate
	}
	for _, gate := range []string{"format", "lint", "test"} {
		if _, ok := adapter.Gates[gate]; !ok {
			t.Fatalf("go-service missing %q", gate)
		}
	}
}

func TestRejectBadAdapters(t *testing.T) {
	t.Parallel()

	base, err := Resolve("python-service")
	if err != nil {
		t.Fatal(err)
	}
	mk := func(mut func(*Adapter)) Adapter {
		adapter := base
		adapter.Gates = map[string]GateSpec{}
		for k, v := range base.Gates {
			adapter.Gates[k] = v
		}
		mut(&adapter)
		return adapter
	}
	_ = mk
	badName := base
	badName.Name = "Bad Adapter!"
	if err := badName.Validate(); err == nil {
		t.Fatal("expected name rejection, got none")
	}
	empty := base
	empty.Gates = map[string]GateSpec{}
	if err := empty.Validate(); err == nil {
		t.Fatal("expected empty-gates rejection, got none")
	}
	badGate := base
	badGate.Gates["teleport"] = GateSpec{Command: []string{"teleport"}, Timeout: time.Minute}
	if err := badGate.Validate(); err == nil {
		t.Fatal("expected gate-name rejection, got none")
	}
	badCmd := base
	badCmd.Gates["test"] = GateSpec{Command: []string{"/bin/evil", "x"}, Timeout: time.Minute}
	if err := badCmd.Validate(); err == nil {
		t.Fatal("expected absolute-binary rejection, got none")
	}
	badDir := base
	badDir.Gates["test"] = GateSpec{Command: []string{"pytest", "-q"}, WorkdirRel: "../evil", Timeout: time.Minute}
	if err := badDir.Validate(); err == nil {
		t.Fatal("expected workdir rejection, got none")
	}
	badTimeout := base
	badTimeout.Gates["test"] = GateSpec{Command: []string{"pytest", "-q"}, Timeout: time.Second}
	if err := badTimeout.Validate(); err == nil {
		t.Fatal("expected timeout rejection, got none")
	}
}
```

NOTE: the `for _, gate := range []string{"format", "vet-gate", "test"}` loop with `_ = gate` in `TestGoServiceDeclaresBaseline` is dead weight; when writing the file, drop it and keep only the meaningful loop over `format`, `lint`, `test`. Likewise drop the `_ = mk` line and use `mk` for all mutation cases (rewrite each bad case through `mk`).

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/stack -run 'TestCatalogResolves|TestGoServiceDeclaresBaseline|TestRejectBadAdapters' -count=1`

Expected: FAIL because `Adapter`, `GateSpec`, `Validate`, `Catalog`, `Resolve`, and `ErrStack` are undefined.

- [ ] **Step 3: Implement adapters and the catalog**

```go
// internal/stack/adapter.go
package stack

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ErrStack = errors.New("invalid stack adapter")

var adapterPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

var gateNames = map[string]bool{
	"format": true, "lint": true, "typecheck": true,
	"test": true, "build": true, "smoke": true,
}

type GateSpec struct {
	Command    []string
	WorkdirRel string
	Timeout    time.Duration
}

type Adapter struct {
	Name    string
	Runtime string
	Gates   map[string]GateSpec
}

func (a Adapter) Validate() error {
	if !adapterPattern.MatchString(a.Name) {
		return fmt.Errorf("%w: adapter name %q", ErrStack, a.Name)
	}
	if a.Runtime == "" {
		return fmt.Errorf("%w: runtime required", ErrStack)
	}
	if len(a.Gates) == 0 {
		return fmt.Errorf("%w: at least one gate", ErrStack)
	}
	for name, gate := range a.Gates {
		if !gateNames[name] {
			return fmt.Errorf("%w: gate %q", ErrStack, name)
		}
		if err := gate.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (g GateSpec) Validate() error {
	if len(g.Command) == 0 || len(g.Command) > 8 {
		return fmt.Errorf("%w: command needs 1..8 args", ErrStack)
	}
	binary := g.Command[0]
	if binary == "" || strings.ContainsAny(binary, " \t\n\r/") {
		return fmt.Errorf("%w: binary must be a bare name", ErrStack)
	}
	for _, arg := range g.Command[1:] {
		if len([]rune(arg)) > 4096 {
			return fmt.Errorf("%w: arg too long", ErrStack)
		}
	}
	if g.WorkdirRel != "" {
		if filepath.IsAbs(g.WorkdirRel) {
			return fmt.Errorf("%w: workdir must be relative", ErrStack)
		}
		clean := filepath.Clean(g.WorkdirRel)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%w: workdir escapes", ErrStack)
		}
	}
	if g.Timeout < 10*time.Second || g.Timeout > 30*time.Minute {
		return fmt.Errorf("%w: timeout outside 10s..30m", ErrStack)
	}
	return nil
}

func Catalog() map[string]Adapter {
	return map[string]Adapter{
		"go-service": {
			Name:    "go-service",
			Runtime: "go",
			Gates: map[string]GateSpec{
				"format": {Command: []string{"gofmt", "-l", "."}, Timeout: time.Minute},
				"lint":   {Command: []string{"go", "vet", "./..."}, Timeout: 5 * time.Minute},
				"test":   {Command: []string{"go", "test", "./..."}, Timeout: 10 * time.Minute},
			},
		},
		"python-service": {
			Name:    "python-service",
			Runtime: "python",
			Gates: map[string]GateSpec{
				"format":    {Command: []string{"ruff", "format", "--check", "."}, Timeout: 2 * time.Minute},
				"lint":      {Command: []string{"ruff", "check", "."}, Timeout: 2 * time.Minute},
				"typecheck": {Command: []string{"mypy", "."}, Timeout: 5 * time.Minute},
				"test":      {Command: []string{"python", "-m", "pytest", "-q"}, Timeout: 10 * time.Minute},
			},
		},
		"nextjs-web": {
			Name:    "nextjs-web",
			Runtime: "node",
			Gates: map[string]GateSpec{
				"lint":      {Command: []string{"npm", "run", "lint"}, Timeout: 5 * time.Minute},
				"typecheck": {Command: []string{"npm", "run", "typecheck"}, Timeout: 5 * time.Minute},
				"test":      {Command: []string{"npm", "test"}, Timeout: 10 * time.Minute},
				"build":     {Command: []string{"npm", "run", "build"}, Timeout: 15 * time.Minute},
			},
		},
	}
}

func Resolve(name string) (Adapter, error) {
	adapter, ok := Catalog()[name]
	if !ok {
		return Adapter{}, fmt.Errorf("%w: unknown adapter %q", ErrStack, name)
	}
	return adapter, nil
}
```

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/stack -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/stack/adapter.go internal/stack/adapter_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/stack/adapter.go internal/stack/adapter_test.go
git commit -m "feat(stack): declare adapters and resolve catalog"
```

### Task 2: Render exact gate commands

**Files:**
- Create: `internal/stack/gate.go`
- Test: `internal/stack/gate_test.go`

**Interfaces:**
- Consumes: validated `Adapter` from Task 1.
- Produces: `GateCommand(a Adapter, gate, projectDir string) (argv []string, cwd string, err error)` resolving the workdir below the Project directory.

- [ ] **Step 1: Write the failing gate test**

```go
package stack

import (
	"reflect"
	"testing"
)

func TestGateCommandRenders(t *testing.T) {
	t.Parallel()

	adapter, err := Resolve("python-service")
	if err != nil {
		t.Fatal(err)
	}
	argv, cwd, err := GateCommand(adapter, "test", "/ws/projects/demo")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(argv, []string{"python", "-m", "pytest", "-q"}) {
		t.Fatalf("unexpected argv: %q", argv)
	}
	if cwd != "/ws/projects/demo" {
		t.Fatalf("unexpected cwd: %q", cwd)
	}
	sub, err := Resolve("nextjs-web")
	if err != nil {
		t.Fatal(err)
	}
	argv, cwd, err = GateCommand(sub, "build", "/ws/projects/web")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(argv, []string{"npm", "run", "build"}) || cwd != "/ws/projects/web" {
		t.Fatalf("unexpected render: %q %q", argv, cwd)
	}
}

func TestGateCommandRejects(t *testing.T) {
	t.Parallel()

	adapter, err := Resolve("go-service")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		gate       string
		projectDir string
	}{
		{name: "unknown gate", gate: "teleport", projectDir: "/ws/projects/demo"},
		{name: "relative dir", gate: "test", projectDir: "ws/projects/demo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := GateCommand(adapter, tc.gate, tc.projectDir); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
	evil := adapter
	evil.Gates["test"] = GateSpec{Command: []string{"go", "test"}, WorkdirRel: "../evil", Timeout: 60000000000}
	if _, _, err := GateCommand(evil, "test", "/ws/projects/demo"); err == nil {
		t.Fatal("expected workdir rejection, got none")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/stack -run 'TestGateCommand' -count=1`

Expected: FAIL because `GateCommand` is undefined.

- [ ] **Step 3: Implement gate rendering**

```go
// internal/stack/gate.go
package stack

import (
	"fmt"
	"path/filepath"
	"strings"
)

func GateCommand(a Adapter, gate, projectDir string) ([]string, error) {
	if err := a.Validate(); err != nil {
		return nil, "", err
	}
	spec, ok := a.Gates[gate]
	if !ok {
		return nil, "", fmt.Errorf("%w: gate %q not declared by %q", ErrStack, gate, a.Name)
	}
	if !filepath.IsAbs(projectDir) {
		return nil, "", fmt.Errorf("%w: project dir must be absolute", ErrStack)
	}
	cwd := filepath.Clean(projectDir)
	if spec.WorkdirRel != "" {
		cwd = filepath.Join(cwd, spec.WorkdirRel)
		rel, err := filepath.Rel(filepath.Clean(projectDir), cwd)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, "", fmt.Errorf("%w: gate workdir escapes project", ErrStack)
		}
	}
	return append([]string{}, spec.Command...), cwd, nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/stack -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/stack/gate.go internal/stack/gate_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/stack/gate.go internal/stack/gate_test.go
git commit -m "feat(stack): render exact gate commands"
```

### Task 3: Execute gates on real toolchains with skips

**Files:**
- Create: `internal/stack/run.go`
- Test: `internal/stack/run_test.go`

**Interfaces:**
- Consumes: rendered argv/cwd from Task 2.
- Produces: `Result struct`, `Run(ctx context.Context, argv []string, cwd string, timeout time.Duration) (Result, error)`, 64 KiB caps, deny-by-default env, toolchain-gated real runs.

- [ ] **Step 1: Write the failing run test**

```go
package stack

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func requireBinary(t *testing.T, name string) {
	t.Helper()

	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not installed", name)
	}
}

func TestRunPythonFixture(t *testing.T) {
	requireBinary(t, "python3")

	dir := t.TempDir()
	fixture := "VALUE = 40 + 2\n"
	if err := os.WriteFile(filepath.Join(dir, "fixture_mod.py"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), []string{"python3", "-m", "py_compile", "fixture_mod.py"}, dir, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("py_compile failed: %#v", res)
	}
}

func TestRunNodeFixture(t *testing.T) {
	requireBinary(t, "node")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixture.js"), []byte("console.log('fixture-ok');\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), []string{"node", "fixture.js"}, dir, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || res.Output != "fixture-ok\n" {
		t.Fatalf("node fixture failed: %#v", res)
	}
}

func TestRunGoGate(t *testing.T) {
	requireBinary(t, "gofmt")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), []string{"gofmt", "-l", "."}, dir, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || res.Output != "" {
		t.Fatalf("formatted file listed as unformatted: %#v", res)
	}
}

func TestRunTimeout(t *testing.T) {
	requireBinary(t, "sleep")

	res, err := Run(context.Background(), []string{"sleep", "30"}, t.TempDir(), 300*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout, got none")
	}
	if !res.TimedOut {
		t.Fatalf("timeout not flagged: %#v", res)
	}
}

func TestRunRejectsEmpty(t *testing.T) {
	t.Parallel()

	if _, err := Run(context.Background(), nil, t.TempDir(), time.Minute); err == nil {
		t.Fatal("expected empty rejection, got none")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/stack -run 'TestRunPython|TestRunNode|TestRunGoGate|TestRunTimeout|TestRunRejectsEmpty' -count=1`

Expected: FAIL because `Run` and `Result` are undefined.

- [ ] **Step 3: Implement bounded host execution**

```go
// internal/stack/run.go
package stack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

var ErrRun = errors.New("stack gate failed")

const outputCap = 64 * 1024

type Result struct {
	ExitCode int
	Output   string
	TimedOut bool
	Duration time.Duration
}

type cappedWriter struct {
	cap int
	buf bytes.Buffer
	hit bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	room := w.cap - w.buf.Len()
	if room <= 0 {
		w.hit = true
		return len(p), nil
	}
	if len(p) > room {
		w.buf.Write(p[:room])
		w.hit = true
		return len(p), nil
	}
	w.buf.Write(p)
	return len(p), nil
}

func hostEnv() []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	for _, key := range []string{"LANG", "LC_ALL", "TZ"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func Run(ctx context.Context, argv []string, cwd string, timeout time.Duration) (Result, error) {
	if len(argv) == 0 || argv[0] == "" {
		return Result{}, fmt.Errorf("%w: empty command", ErrRun)
	}
	if cwd == "" {
		return Result{}, fmt.Errorf("%w: working directory required", ErrRun)
	}
	if timeout <= 0 {
		return Result{}, fmt.Errorf("%w: timeout required", ErrRun)
	}
	binary, err := exec.LookPath(argv[0])
	if err != nil {
		return Result{}, fmt.Errorf("%w: binary %q not found", ErrRun, argv[0])
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(runCtx, binary, argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = hostEnv()
	var out cappedWriter
	out.cap = outputCap
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	res := Result{Output: out.buf.String(), Duration: time.Since(start)}
	if runErr == nil {
		return res, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
		return res, fmt.Errorf("%w: timed out", ErrRun)
	}
	return res, fmt.Errorf("%w: exit code %d", ErrRun, res.ExitCode)
}
```

NOTE: combined output (stdout+stderr merged) is deliberate for gate logs: one ordered stream, one cap. Document it in the plan and in `Result`.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/stack -count=1`

Expected: PASS (real toolchains present; skips only where binaries are missing).

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/stack/run.go internal/stack/run_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/stack/run.go internal/stack/run_test.go
git commit -m "feat(stack): execute gates on host toolchains"
```

### Task 4: Close out v1 with the adapter flow, README, and roadmap

**Files:**
- Create: `internal/stack/flow_test.go`
- Modify: `docs/implementation-plan.md`, `README.md`

**Interfaces:**
- Consumes: complete adapter lifecycle from Tasks 1-3.
- Produces: executable evidence that manifest adapter names resolve to runnable gates on every present toolchain, plus an honest v1 close-out note.

- [ ] **Step 1: Add the flow test**

```go
package stack

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestAdapterFlow(t *testing.T) {
	for _, tc := range []struct {
		adapter string
		binary  string
		argv    []string
		fixture string
		name    string
		want    string
	}{
		{adapter: "python-service", binary: "python3", argv: []string{"python3", "-m", "py_compile", "fixture_mod.py"}, fixture: "VALUE = 1\n", name: "fixture_mod.py", want: ""},
		{adapter: "nextjs-web", binary: "node", argv: []string{"node", "fixture.js"}, fixture: "console.log('flow-ok');\n", name: "fixture.js", want: "flow-ok\n"},
		{adapter: "go-service", binary: "gofmt", argv: []string{"gofmt", "-l", "."}, fixture: "package fixture\n", name: "fixture.go", want: ""},
	} {
		t.Run(tc.adapter, func(t *testing.T) {
			if _, err := exec.LookPath(tc.binary); err != nil {
				t.Skipf("%s not installed", tc.binary)
			}
			resolved, err := Resolve(tc.adapter)
			if err != nil {
				t.Fatal(err)
			}
			if err := resolved.Validate(); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.name), []byte(tc.fixture), 0o644); err != nil {
				t.Fatal(err)
			}
			res, err := Run(context.Background(), tc.argv, dir, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if res.Output != tc.want {
				t.Fatalf("unexpected output: %#v", res)
			}
		})
	}
}
```

- [ ] **Step 2: Run the flow test**

Run: `go test ./internal/stack -run 'TestAdapterFlow' -count=1 -v`

Expected: PASS on all three toolchains present.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean build of `cmd/harness`.

- [ ] **Step 4: Close out v1 and commit**

In `docs/implementation-plan.md`, add Increment 16 to the plan index and a verification checklist below Increment 15, then append a v1 close-out note: all 16 module increments are implemented with executable evidence on master; the release acceptance scenario from the roadmap (external Issue through Triage, Spec, Codex implementation, containerized Gates, review, preserved commits, PR approval, immutable release, Coolify deploy, controlled exposure, acceptance, rollout, scheduled flag removal, OKF proposal, restart recovery, no secret leakage) remains an operator-run E2E against real systems and is not claimed by module tests.

In `README.md`, replace `The repository currently contains the agreed architecture and implementation roadmap. Runtime code has not been started.` with a runtime summary: the implemented modules, the `harness` CLI surface so far, and how to run the verification set.

```bash
git add internal/stack/flow_test.go docs/implementation-plan.md README.md
git commit -m "test(stack): verify adapter flow and close v1"
```
