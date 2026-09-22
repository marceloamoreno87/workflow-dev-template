# Workspace Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prepare and close isolated Project worktrees with generated Git configuration that disables Project hooks and credential helpers and never leaks host credentials into the Execution environment.

**Architecture:** `workspace` owns worktree request validation, safe Git config generation, sanitized process environments, and Prepare/Close against the real Git binary; it never touches the network. It may import nothing outside the standard library. `workflow`, `gatekeeper`, `journal`, `registry`, `cli`, and `github` stay untouched with no new imports. Ancestry/scope verification and push authentication belong to the daemon (later increment) and are explicitly out of scope.

**Tech Stack:** Go 1.27.1 standard library only (`os`, `os/exec`, `path/filepath`, `strings`, real `git` binary in tests, table-driven tests)

**Spec:** `docs/architecture.md` (Module Workspace prepares/closes isolated work; repository shape `internal/workspace`; no utils/common/manager), `docs/adr/0002-link-projects-and-isolate-executions.md` (Project is a submodule; Execution work happens in an isolated worktree; routine execution never silently moves the registered submodule reference), `docs/adr/0030-isolate-git-effects-and-network-credentials.md` (generated repository configuration with Project hooks and global includes disabled; commit-time checks are explicit Gates), `docs/security.md` (credentials remain outside Project worktrees; generated Git config, empty hooks path)

## Global Constraints

- Use Go 1.27.1.
- Standard library only; do not add dependencies.
- `internal/workspace` imports nothing outside the standard library.
- Use the canonical terms from `CONTEXT.md` (Harness Workspace, Project, Project Worktree, Execution); never write clone, sandbox, or branch for a Project Worktree.
- The Project Worktree root is always `<workspace>/.worktrees/<project-id>`; Project ids match `[A-Za-z0-9._-]{1,100}` and every path resolves below the Harness Workspace root.
- The worktree is added detached at the submodule HEAD so routine execution cannot move a branch the workspace depends on; the registered submodule reference (gitlink) must be unchanged after Prepare, local commits, and Close.
- Generated Git configuration sets harness identity, an empty controlled hooks path, and a cleared credential helper. System and global includes are disabled at runtime via environment, never by editing host files.
- Every Git subprocess runs with a deny-by-default environment built from an explicit allowlist; secret-bearing variables (`*TOKEN*`, `*SECRET*`, `*PASSWORD*`, `*CREDENTIALS*`, `*PRIVATE_KEY*`) never pass through, and the `file` transport is denied at runtime.
- Tests use the real `git` binary in `t.TempDir()` fixtures with `protocol.file.allow` enabled only for explicit fixture setup commands, never in generated configuration or Prepared environments.
- Do not add generic repository, provider, manager, service, utils, or common packages; new code lives in `internal/workspace` only.

---

### Task 1: Validate the Project Worktree request

**Files:**
- Create: `internal/workspace/workspace.go`
- Test: `internal/workspace/workspace_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `Request struct`, `WorktreeDir(root, projectID string) (string, error)`, `ValidateRequest(root, projectID, submodulePath string) error`, sentinel `ErrWorkspace`.

- [ ] **Step 1: Write the failing request test**

```go
package workspace

import (
	"path/filepath"
	"testing"
)

func TestWorktreeDirConfinesProject(t *testing.T) {
	t.Parallel()

	dir, err := WorktreeDir("/ws", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join("/ws", ".worktrees", "demo") {
		t.Fatalf("unexpected dir: %q", dir)
	}
}

func TestRejectBadRequests(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		root          string
		projectID     string
		submodulePath string
	}{
		{name: "relative root", root: "ws", projectID: "demo", submodulePath: "/ws/projects/demo"},
		{name: "empty id", root: "/ws", projectID: "", submodulePath: "/ws/projects/demo"},
		{name: "slash id", root: "/ws", projectID: "../evil", submodulePath: "/ws/projects/evil"},
		{name: "space id", root: "/ws", projectID: "de mo", submodulePath: "/ws/projects/de mo"},
		{name: "escape submodule", root: "/ws", projectID: "demo", submodulePath: "/ws/../evil"},
		{name: "outside submodule", root: "/ws", projectID: "demo", submodulePath: "/other/demo"},
		{name: "relative submodule", root: "/ws", projectID: "demo", submodulePath: "projects/demo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRequest(tc.root, tc.projectID, tc.submodulePath); err == nil {
				t.Fatal("expected rejection, got none")
			}
			if _, err := WorktreeDir(tc.root, tc.projectID); tc.name == "slash id" || tc.name == "space id" || tc.name == "empty id" {
				if err == nil {
					t.Fatal("expected WorktreeDir rejection, got none")
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/workspace -run 'TestWorktreeDir|TestRejectBadRequests' -count=1`

Expected: FAIL because `WorktreeDir`, `ValidateRequest`, and `ErrWorkspace` are undefined.

- [ ] **Step 3: Implement request validation with path confinement**

```go
// internal/workspace/workspace.go
package workspace

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var ErrWorkspace = errors.New("invalid workspace request")

type Request struct {
	WorkspaceRoot string
	ProjectID     string
	SubmodulePath string
}

func validProjectID(id string) bool {
	if len(id) == 0 || len(id) > 100 {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func cleanAbs(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%w: path %q is not absolute", ErrWorkspace, p)
	}
	return filepath.Clean(p), nil
}

func WorktreeDir(root, projectID string) (string, error) {
	abs, err := cleanAbs(root)
	if err != nil {
		return "", err
	}
	if !validProjectID(projectID) {
		return "", fmt.Errorf("%w: project id %q", ErrWorkspace, projectID)
	}
	return filepath.Join(abs, ".worktrees", projectID), nil
}

func belowRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func ValidateRequest(root, projectID, submodulePath string) error {
	abs, err := cleanAbs(root)
	if err != nil {
		return err
	}
	if !validProjectID(projectID) {
		return fmt.Errorf("%w: project id %q", ErrWorkspace, projectID)
	}
	sub, err := cleanAbs(submodulePath)
	if err != nil {
		return err
	}
	if !belowRoot(abs, sub) || sub == abs {
		return fmt.Errorf("%w: submodule path %q escapes workspace", ErrWorkspace, submodulePath)
	}
	return nil
}
```

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/workspace -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/workspace/workspace.go internal/workspace/workspace_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/workspace/workspace.go internal/workspace/workspace_test.go
git commit -m "feat(workspace): validate project worktree requests"
```

### Task 2: Generate the safe Git configuration

**Files:**
- Create: `internal/workspace/config.go`
- Test: `internal/workspace/config_test.go`

**Interfaces:**
- Consumes: `Request` validation from Task 1.
- Produces: `Prepared struct`, `WriteGitConfig(worktreeDir string) (Prepared, error)` writing `<worktreeDir>/.harness-git/config` plus an empty `<worktreeDir>/.harness-git/hooks` directory.

- [ ] **Step 1: Write the failing config test**

```go
package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteGitConfigDisablesHooksAndHelpers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prepared, err := WriteGitConfig(filepath.Join(dir, ".worktrees", "demo"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(prepared.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(raw)
	if !strings.Contains(cfg, "hooksPath") || !strings.Contains(cfg, prepared.HooksDir) {
		t.Fatalf("hooksPath missing: %q", cfg)
	}
	if !strings.Contains(cfg, "[credential]") {
		t.Fatalf("credential section missing: %q", cfg)
	}
	info, err := os.Stat(prepared.HooksDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("hooks dir missing: %v", err)
	}
	entries, err := os.ReadDir(prepared.HooksDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("hooks dir must be empty: %v %v", entries, err)
	}
}

func TestWriteGitConfigRejectsBadDir(t *testing.T) {
	t.Parallel()

	if _, err := WriteGitConfig("relative/dir"); err == nil {
		t.Fatal("expected rejection of relative dir, got none")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/workspace -run 'TestWriteGitConfig' -count=1`

Expected: FAIL because `WriteGitConfig` and `Prepared` are undefined.

- [ ] **Step 3: Implement safe config generation**

```go
// internal/workspace/config.go
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

type Prepared struct {
	Dir        string
	ConfigFile string
	HooksDir   string
}

func WriteGitConfig(worktreeDir string) (Prepared, error) {
	abs, err := cleanAbs(worktreeDir)
	if err != nil {
		return Prepared{}, err
	}
	gitDir := filepath.Join(abs, ".harness-git")
	hooks := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		return Prepared{}, err
	}
	configFile := filepath.Join(gitDir, "config")
	cfg := "[user]\n\tname = harness\n\temail = harness@localhost\n[core]\n\thooksPath = " + hooks + "\n[credential]\n\thelper = \n"
	if err := os.WriteFile(configFile, []byte(cfg), 0o644); err != nil {
		return Prepared{}, err
	}
	return Prepared{Dir: abs, ConfigFile: configFile, HooksDir: hooks}, nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/workspace -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/workspace/config.go internal/workspace/config_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/workspace/config.go internal/workspace/config_test.go
git commit -m "feat(workspace): generate safe git configuration"
```

### Task 3: Prepare and Close the Project Worktree round-trip

**Files:**
- Create: `internal/workspace/prepare.go`
- Test: `internal/workspace/prepare_test.go`

**Interfaces:**
- Consumes: `ValidateRequest`, `WorktreeDir`, `WriteGitConfig` from Tasks 1-2.
- Produces: `Prepare(root, projectID, submodulePath string) (Prepared, error)`, `Close(root, projectID, submodulePath string) error`.

- [ ] **Step 1: Write the failing round-trip test**

```go
package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitFixture(t *testing.T, args []string, dir string, extra ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, extra...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func fixtureWorkspace(t *testing.T) (root, submodule string) {
	t.Helper()

	root = t.TempDir()
	src := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, []string{"init", "-b", "main"}, src)
	gitFixture(t, []string{"config", "user.name", "fixture"}, src)
	gitFixture(t, []string{"config", "user.email", "fixture@localhost"}, src)
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, []string{"add", "README.md"}, src)
	gitFixture(t, []string{"commit", "-m", "initial"}, src)

	gitFixture(t, []string{"init", "-b", "main"}, root)
	gitFixture(t, []string{"config", "user.name", "fixture"}, root)
	gitFixture(t, []string{"config", "user.email", "fixture@localhost"}, root)
	gitFixture(t, []string{"-c", "protocol.file.allow=always", "submodule", "add", src, "projects/demo"}, root)
	gitFixture(t, []string{"commit", "-m", "link demo"}, root)
	return root, filepath.Join(root, "projects", "demo")
}

func gitlink(t *testing.T, root string) string {
	t.Helper()

	return strings.TrimSpace(gitFixture(t, []string{"rev-parse", "HEAD:projects/demo"}, root))
}

func TestPrepareCloseRoundTrip(t *testing.T) {
	t.Parallel()

	root, submodule := fixtureWorkspace(t)
	before := gitlink(t, root)

	prepared, err := Prepare(root, "demo", submodule)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(prepared.Dir, "README.md")); err != nil {
		t.Fatalf("worktree checkout missing: %v", err)
	}
	head := strings.TrimSpace(gitFixture(t, []string{"rev-parse", "HEAD"}, prepared.Dir, prepared.Env()...))
	sub := strings.TrimSpace(gitFixture(t, []string{"rev-parse", "HEAD"}, submodule))
	if head != sub {
		t.Fatalf("worktree HEAD %q != submodule HEAD %q", head, sub)
	}

	gitFixture(t, []string{"commit", "--allow-empty", "-m", "execution work"}, prepared.Dir, prepared.Env()...)
	if after := gitlink(t, root); after != before {
		t.Fatalf("registered submodule reference moved: %q -> %q", before, after)
	}

	if err := Close(root, "demo", submodule); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(prepared.Dir); !os.IsNotExist(err) {
		t.Fatalf("worktree dir survives close: %v", err)
	}
	list := gitFixture(t, []string{"worktree", "list", "--porcelain"}, submodule)
	if strings.Contains(list, prepared.Dir) {
		t.Fatalf("worktree still registered: %q", list)
	}
	if after := gitlink(t, root); after != before {
		t.Fatalf("registered submodule reference moved by close: %q -> %q", before, after)
	}
}

func TestPrepareRejectsExistingDir(t *testing.T) {
	t.Parallel()

	root, submodule := fixtureWorkspace(t)
	if _, err := Prepare(root, "demo", submodule); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Close(root, "demo", submodule); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := Prepare(root, "demo", submodule); err == nil {
		t.Fatal("expected rejection of existing worktree dir, got none")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/workspace -run 'TestPrepare' -count=1`

Expected: FAIL because `Prepare`, `Close`, and `Env` are undefined.

- [ ] **Step 3: Implement Prepare/Close with detached worktrees and sanitized env**

```go
// internal/workspace/prepare.go
package workspace

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func baseEnv() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_TERMINAL_PROMPT=0",
	}
	for _, key := range []string{"LANG", "LC_ALL", "TZ"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func (p Prepared) Env() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_GLOBAL=" + p.ConfigFile,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ALLOW_PROTOCOL=https:ssh:",
	}
	for _, key := range []string{"LANG", "LC_ALL", "TZ"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func runGit(dir string, env []string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := out.String()
		if len(msg) > 2048 {
			msg = msg[:2048]
		}
		return fmt.Errorf("%w: git %s: %v: %s", ErrWorkspace, strings.Join(args, " "), err, msg)
	}
	return nil
}

func Prepare(root, projectID, submodulePath string) (Prepared, error) {
	if err := ValidateRequest(root, projectID, submodulePath); err != nil {
		return Prepared{}, err
	}
	dir, err := WorktreeDir(root, projectID)
	if err != nil {
		return Prepared{}, err
	}
	if _, err := os.Stat(dir); err == nil {
		return Prepared{}, fmt.Errorf("%w: worktree dir %q exists, close it first", ErrWorkspace, dir)
	}
	if err := os.MkdirAll(filepath.Join(root, ".worktrees"), 0o755); err != nil {
		return Prepared{}, err
	}
	if err := runGit(submodulePath, baseEnv(), "worktree", "add", "--detach", dir, "HEAD"); err != nil {
		return Prepared{}, err
	}
	prepared, err := WriteGitConfig(dir)
	if err != nil {
		_ = runGit(submodulePath, baseEnv(), "worktree", "remove", "--force", dir)
		return Prepared{}, err
	}
	return prepared, nil
}

func Close(root, projectID, submodulePath string) error {
	if err := ValidateRequest(root, projectID, submodulePath); err != nil {
		return err
	}
	dir, err := WorktreeDir(root, projectID)
	if err != nil {
		return err
	}
	if err := runGit(submodulePath, baseEnv(), "worktree", "remove", "--force", dir); err != nil {
		return err
	}
	_ = runGit(submodulePath, baseEnv(), "worktree", "prune")
	if _, err := os.Stat(dir); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/workspace -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/workspace/prepare.go internal/workspace/prepare_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/workspace/prepare.go internal/workspace/prepare_test.go
git commit -m "feat(workspace): prepare and close project worktrees"
```

### Task 4: Prove environment sanitization, hook isolation, and credential absence

**Files:**
- Create: `internal/workspace/isolation_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete `Prepare`/`Close`/`Env` from Task 3 plus the Task 3 fixture helpers.
- Produces: executable evidence that secret-bearing variables never enter the Execution environment, Project hooks never run, and no credential helper survives.

- [ ] **Step 1: Add the isolation test**

```go
package workspace

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPrepareSanitizesEnvironment(t *testing.T) {
	t.Parallel()

	t.Setenv("GH_TOKEN", "host-secret")
	t.Setenv("GITHUB_TOKEN", "host-secret")
	t.Setenv("COOLIFY_TOKEN", "host-secret")
	t.Setenv("TELEGRAM_BOT_TOKEN", "host-secret")
	t.Setenv("MY_SECRET_VALUE", "host-secret")
	t.Setenv("DB_PASSWORD", "host-secret")

	root, submodule := fixtureWorkspace(t)
	prepared, err := Prepare(root, "demo", submodule)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Close(root, "demo", submodule); err != nil {
			t.Fatal(err)
		}
	})
	for _, entry := range prepared.Env() {
		upper := strings.ToUpper(entry)
		if strings.Contains(entry, "host-secret") {
			t.Fatalf("secret value leaked into env: %q", entry)
		}
		for _, banned := range []string{"GH_TOKEN", "GITHUB_TOKEN", "COOLIFY_TOKEN", "TELEGRAM_BOT_TOKEN", "MY_SECRET_VALUE", "DB_PASSWORD"} {
			if strings.HasPrefix(upper, banned+"=") {
				t.Fatalf("secret variable leaked into env: %q", entry)
			}
		}
	}
	joined := strings.Join(prepared.Env(), "\n")
	for _, want := range []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_ALLOW_PROTOCOL=https:ssh:"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("hardening %q missing from env:\n%s", want, joined)
		}
	}
}

func TestPreparedHooksAndHelpersAreDisabled(t *testing.T) {
	t.Parallel()

	root, submodule := fixtureWorkspace(t)
	prepared, err := Prepare(root, "demo", submodule)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Close(root, "demo", submodule); err != nil {
			t.Fatal(err)
		}
	})
	cmd := exec.Command("git", "config", "--list")
	cmd.Dir = prepared.Dir
	cmd.Env = prepared.Env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git config --list: %v\n%s", err, out)
	}
	list := string(out)
	if !strings.Contains(list, "core.hookspath="+prepared.HooksDir) {
		t.Fatalf("hooksPath not enforced:\n%s", list)
	}
	for _, line := range strings.Split(list, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "credential.helper") && len(strings.TrimSpace(strings.TrimPrefix(lower, "credential.helper="))) > 0 {
			t.Fatalf("credential helper survives: %q", line)
		}
	}
}
```

- [ ] **Step 2: Run the isolation test**

Run: `go test ./internal/workspace -run 'TestPrepareSanitizes|TestPreparedHooks' -count=1`

Expected: PASS (prepare.go already implements the sanitized env; the test locks it).

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean build of `cmd/harness`.

- [ ] **Step 4: Mark Increment 5 verified and commit**

In `docs/implementation-plan.md`, add Increment 5 to the plan index and a verification checklist below Increment 4. Do not mark it complete until the commands above pass on master.

```bash
git add internal/workspace/isolation_test.go docs/implementation-plan.md
git commit -m "test(workspace): verify isolation and credential absence"
```
