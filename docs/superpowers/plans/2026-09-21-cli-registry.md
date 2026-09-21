# CLI and Registry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Register Projects in a versioned registry, validate Project manifests against structured contracts, and inspect the workspace through an automatable CLI.

**Architecture:** `registry` owns manifest and registry parsing plus validation as pure functions on bytes and strings; it never touches the filesystem or network. `cli` owns filesystem access and exit codes, calling into `registry` and writing results to provided writers. `cmd/harness` is a thin `main` wrapper. `workflow`, `gatekeeper`, and `journal` stay untouched with no new imports.

**Tech Stack:** Go 1.27.1 standard library plus `gopkg.in/yaml.v3` (manifest and registry documents are YAML per contracts), `flag` package for subcommands, table-driven tests

**Spec:** `docs/contracts.md` (project manifest, workspace registry, schemaVersion), `docs/architecture.md` (repository shape `cmd/harness`, no utils/common/manager), `docs/adr/0002-link-projects-and-isolate-executions.md`, `docs/adr/0026-version-structured-artifacts.md`

## Global Constraints

- Use Go 1.27.1.
- Keep `workflow`, `gatekeeper`, and `journal` free of CLI, YAML, and filesystem imports; only `registry` may import the YAML driver and only `cli`/`cmd/harness` touch the filesystem.
- Use the canonical terms from `CONTEXT.md` (Project, Harness Workspace, Work Item, Spec, Quality Profile, Data Classification); never write folder, package, module, task, ticket, or branch for those meanings.
- Every persisted manifest and registry document carries an explicit `schemaVersion`; readers reject unknown or missing versions.
- Registry paths must resolve below `projects/`, never escape via absolute paths or `..`, and must agree with the submodule record.
- The manifest contains policy and non-secret references only; secret-looking keys are rejected.
- Do not add generic repository, provider, manager, service, utils, or common packages; new code lives in `internal/registry`, `internal/cli`, and `cmd/harness` only.

---

### Task 1: Parse and validate the Project manifest

**Files:**
- Create: `internal/registry/project.go`
- Test: `internal/registry/project_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `ProjectManifest struct`, `ParseProjectManifest(data []byte) (ProjectManifest, error)`, `(ProjectManifest).Validate() error`, `RejectSecrets(data []byte) error`, sentinels `ErrSchemaVersion`, `ErrManifest`, `ErrSecret`.

- [ ] **Step 1: Write the failing manifest test**

```go
package registry

import (
	"testing"
)

const validManifest = `schemaVersion: harness/v1
project:
  id: example
  repository: owner/repo
  defaultBranch: main
stack:
  adapter: go-service
quality:
  profile: standard
data:
  classification: internal
knowledge:
  root: .knowledge
specs:
  root: .specs
delivery:
  coolifyResource: resource-id
  productionUrl: https://example.com
flags:
  provider: go-feature-flag
  declarationRoot: .flags
budgets:
  maxDuration: 2h
  maxCostUSD: 10
  maxFixCycles: 3
`

func TestParseValidManifest(t *testing.T) {
	t.Parallel()

	m, err := ParseProjectManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	if m.Project.ID != "example" || m.Project.Repository != "owner/repo" {
		t.Fatalf("unexpected manifest: %#v", m.Project)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := RejectSecrets([]byte(validManifest)); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadManifests(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		doc  string
	}{
		{name: "missing schema", doc: "project:\n  id: x\n"},
		{name: "wrong schema", doc: "schemaVersion: harness/v9\nproject:\n  id: x\n  repository: o/r\n"},
		{name: "bad profile", doc: "schemaVersion: harness/v1\nproject:\n  id: x\n  repository: o/r\nstack:\n  adapter: go-service\nquality:\n  profile: turbo\ndata:\n  classification: internal\n"},
		{name: "bad classification", doc: "schemaVersion: harness/v1\nproject:\n  id: x\n  repository: o/r\nstack:\n  adapter: go-service\nquality:\n  profile: standard\ndata:\n  classification: topsecret\n"},
		{name: "secret key", doc: "schemaVersion: harness/v1\nproject:\n  id: x\n  repository: o/r\ndelivery:\n  apiToken: abc\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := ParseProjectManifest([]byte(tc.doc))
			if err == nil {
				if verr := m.Validate(); verr == nil {
					if serr := RejectSecrets([]byte(tc.doc)); serr == nil {
						t.Fatal("expected rejection, got none")
					}
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/registry -run 'TestParseValidManifest|TestRejectBadManifests' -count=1`

Expected: FAIL because `ParseProjectManifest` and the sentinels are undefined.

- [ ] **Step 3: Add the YAML dependency and minimal manifest code**

Run: `go get gopkg.in/yaml.v3@latest`

```go
// internal/registry/project.go
package registry

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrSchemaVersion = errors.New("unsupported schema version")
	ErrManifest      = errors.New("invalid project manifest")
	ErrSecret        = errors.New("secret material in manifest")
)

type ProjectManifest struct {
	SchemaVersion string `yaml:"schemaVersion"`
	Project       struct {
		ID            string `yaml:"id"`
		Repository    string `yaml:"repository"`
		DefaultBranch string `yaml:"defaultBranch"`
	} `yaml:"project"`
	Stack struct {
		Adapter string `yaml:"adapter"`
	} `yaml:"stack"`
	Quality struct {
		Profile string `yaml:"profile"`
	} `yaml:"quality"`
	Data struct {
		Classification string `yaml:"classification"`
	} `yaml:"data"`
}

func ParseProjectManifest(data []byte) (ProjectManifest, error) {
	var m ProjectManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return ProjectManifest{}, err
	}
	if m.SchemaVersion != "harness/v1" {
		return ProjectManifest{}, fmt.Errorf("%w: %q", ErrSchemaVersion, m.SchemaVersion)
	}
	return m, nil
}

func (m ProjectManifest) Validate() error {
	if m.Project.ID == "" || m.Project.Repository == "" {
		return fmt.Errorf("%w: project id and repository are required", ErrManifest)
	}
	if m.Stack.Adapter == "" {
		return fmt.Errorf("%w: stack adapter is required", ErrManifest)
	}
	switch m.Quality.Profile {
	case "prototype", "standard", "critical":
	default:
		return fmt.Errorf("%w: quality profile %q", ErrManifest, m.Quality.Profile)
	}
	switch m.Data.Classification {
	case "public", "internal", "confidential", "restricted":
	default:
		return fmt.Errorf("%w: data classification %q", ErrManifest, m.Data.Classification)
	}
	return nil
}

var secretKeys = map[string]bool{
	"password": true, "secret": true, "token": true, "apikey": true,
	"api_key": true, "privatekey": true, "private_key": true, "credentials": true,
}

func RejectSecrets(data []byte) error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	return rejectSecretsMap(raw)
}

func rejectSecretsMap(m map[string]any) error {
	for k, v := range m {
		flat := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(k, "_", ""), "-", ""))
		if secretKeys[flat] {
			return fmt.Errorf("%w: key %q", ErrSecret, k)
		}
		if nested, ok := v.(map[string]any); ok {
			if err := rejectSecretsMap(nested); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/registry -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/registry/project.go internal/registry/project_test.go && go test ./...`

Expected: PASS.

```bash
git add go.mod go.sum internal/registry/project.go internal/registry/project_test.go
git commit -m "feat(registry): parse and validate project manifests"
```

### Task 2: Parse and validate the workspace registry

**Files:**
- Create: `internal/registry/registry.go`
- Modify: `internal/registry/project_test.go` (no changes; new test file below)
- Test: `internal/registry/registry_test.go`

**Interfaces:**
- Consumes: `ProjectManifest` field names from Task 1 (for later consistency task).
- Produces: `Registry struct`, `RegistryProject struct`, `ParseRegistry(data []byte) (Registry, error)`, `(Registry).Validate() error`.

- [ ] **Step 1: Write the failing registry test**

```go
package registry

import (
	"testing"
)

const validRegistry = `schemaVersion: harness.registry/v1
projects:
  - id: example
    path: projects/example
    github:
      repository: owner/repo
      clientProject: PVT_client
      portfolioProject: PVT_private
    actors:
      operator: actor/operator
      clients: [actor/client-a]
`

func TestParseValidRegistry(t *testing.T) {
	t.Parallel()

	r, err := ParseRegistry([]byte(validRegistry))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Projects) != 1 || r.Projects[0].ID != "example" {
		t.Fatalf("unexpected registry: %#v", r.Projects)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadRegistries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  string
	}{
		{name: "missing schema", doc: "projects: []\n"},
		{name: "wrong schema", doc: "schemaVersion: harness/v9\nprojects: []\n"},
		{name: "absolute path", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: /etc/passwd\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n"},
		{name: "escape", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: projects/../secret\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n"},
		{name: "outside projects", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: other/a\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n"},
		{name: "duplicate id", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: projects/a\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n  - id: a\n    path: projects/b\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n"},
		{name: "missing repository", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: projects/a\n    github: {}\n    actors:\n      operator: actor/operator\n"},
		{name: "missing operator", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: projects/a\n    github:\n      repository: o/r\n    actors: {}\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ParseRegistry([]byte(tc.doc))
			if err != nil {
				return
			}
			if err := r.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/registry -run 'TestParseValidRegistry|TestRejectBadRegistries' -count=1`

Expected: FAIL because `ParseRegistry` is undefined.

- [ ] **Step 3: Implement registry parsing with path confinement**

```go
// internal/registry/registry.go
package registry

import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type Registry struct {
	SchemaVersion string            `yaml:"schemaVersion"`
	Projects      []RegistryProject `yaml:"projects"`
}

type RegistryProject struct {
	ID     string `yaml:"id"`
	Path   string `yaml:"path"`
	Github struct {
		Repository       string `yaml:"repository"`
		ClientProject    string `yaml:"clientProject"`
		PortfolioProject string `yaml:"portfolioProject"`
	} `yaml:"github"`
	Actors struct {
		Operator string   `yaml:"operator"`
		Clients  []string `yaml:"clients"`
	} `yaml:"actors"`
}

func ParseRegistry(data []byte) (Registry, error) {
	var r Registry
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Registry{}, err
	}
	if r.SchemaVersion != "harness.registry/v1" {
		return Registry{}, fmt.Errorf("%w: %q", ErrSchemaVersion, r.SchemaVersion)
	}
	return r, nil
}

func (r Registry) Validate() error {
	ids := map[string]bool{}
	paths := map[string]bool{}
	for _, p := range r.Projects {
		if p.ID == "" {
			return fmt.Errorf("%w: project id is required", ErrManifest)
		}
		if ids[p.ID] {
			return fmt.Errorf("%w: duplicate project id %q", ErrManifest, p.ID)
		}
		ids[p.ID] = true
		clean := path.Clean(p.Path)
		if path.IsAbs(p.Path) || clean != p.Path || !strings.HasPrefix(clean, "projects/") || clean == "projects/" || strings.Contains(clean, "..") {
			return fmt.Errorf("%w: path %q must resolve below projects/", ErrManifest, p.Path)
		}
		if paths[clean] {
			return fmt.Errorf("%w: duplicate project path %q", ErrManifest, p.Path)
		}
		paths[clean] = true
		if p.Github.Repository == "" {
			return fmt.Errorf("%w: github repository is required for %q", ErrManifest, p.ID)
		}
		if p.Actors.Operator == "" {
			return fmt.Errorf("%w: operator actor is required for %q", ErrManifest, p.ID)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/registry -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/registry/registry.go internal/registry/registry_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/registry/registry.go internal/registry/registry_test.go
git commit -m "feat(registry): parse and validate workspace registry"
```

### Task 3: Prove registry, manifest, and submodule agreement

**Files:**
- Create: `internal/registry/consistency.go`
- Test: `internal/registry/consistency_test.go`

**Interfaces:**
- Consumes: `Registry`, `ProjectManifest` from Tasks 1-2.
- Produces: `CheckConsistency(reg Registry, manifests map[string]ProjectManifest, gitmodules string) error`.

- [ ] **Step 1: Write the failing consistency test**

```go
package registry

import (
	"errors"
	"testing"
)

const exampleGitmodules = `[submodule "projects/example"]
	path = projects/example
	url = git@github.com:owner/repo.git
`

func TestConsistencyAcceptsMatchingWorkspace(t *testing.T) {
	t.Parallel()

	r, err := ParseRegistry([]byte(validRegistry))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseProjectManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckConsistency(r, map[string]ProjectManifest{"example": m}, exampleGitmodules); err != nil {
		t.Fatal(err)
	}
}

func TestConsistencyRejectsMismatches(t *testing.T) {
	t.Parallel()

	r, err := ParseRegistry([]byte(validRegistry))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseProjectManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("missing manifest", func(t *testing.T) {
		if err := CheckConsistency(r, map[string]ProjectManifest{}, exampleGitmodules); err == nil {
			t.Fatal("expected missing manifest error")
		}
	})

	t.Run("id mismatch", func(t *testing.T) {
		other := m
		other.Project.ID = "renamed"
		if err := CheckConsistency(r, map[string]ProjectManifest{"example": other}, exampleGitmodules); err == nil {
			t.Fatal("expected id mismatch error")
		}
	})

	t.Run("repository mismatch", func(t *testing.T) {
		other := m
		other.Project.Repository = "owner/other"
		err := CheckConsistency(r, map[string]ProjectManifest{"example": other}, exampleGitmodules)
		if err == nil || !errors.Is(err, ErrManifest) {
			t.Fatalf("expected manifest mismatch, got %v", err)
		}
	})

	t.Run("missing submodule", func(t *testing.T) {
		if err := CheckConsistency(r, map[string]ProjectManifest{"example": m}, ""); err == nil {
			t.Fatal("expected submodule error")
		}
	})
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/registry -run TestConsistency -count=1`

Expected: FAIL because `CheckConsistency` is undefined.

- [ ] **Step 3: Implement the agreement check**

```go
// internal/registry/consistency.go
package registry

import (
	"fmt"
	"strings"
)

func CheckConsistency(reg Registry, manifests map[string]ProjectManifest, gitmodules string) error {
	for _, p := range reg.Projects {
		m, ok := manifests[p.ID]
		if !ok {
			return fmt.Errorf("%w: missing manifest for project %q", ErrManifest, p.ID)
		}
		if m.Project.ID != p.ID {
			return fmt.Errorf("%w: manifest id %q disagrees with registry %q", ErrManifest, m.Project.ID, p.ID)
		}
		if m.Project.Repository != p.Github.Repository {
			return fmt.Errorf("%w: manifest repository %q disagrees with registry %q", ErrManifest, m.Project.Repository, p.Github.Repository)
		}
		needle := "path = " + p.Path
		found := false
		for _, line := range strings.Split(gitmodules, "\n") {
			if strings.TrimSpace(line) == needle {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: path %q missing from submodule record", ErrManifest, p.Path)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/registry -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/registry/consistency.go internal/registry/consistency_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/registry/consistency.go internal/registry/consistency_test.go
git commit -m "feat(registry): check registry manifest submodule agreement"
```

### Task 4: Expose registration and inspection through the CLI

**Files:**
- Create: `internal/cli/cli.go`
- Test: `internal/cli/cli_test.go`
- Create: `cmd/harness/main.go`

**Interfaces:**
- Consumes: `registry.ParseRegistry`, `registry.ParseProjectManifest`, `registry.CheckConsistency`, `Validate`, `RejectSecrets` from Tasks 1-3.
- Produces: `Run(args []string, stdout, stderr io.Writer) int` with subcommands `validate`, `list`, `register`.

- [ ] **Step 1: Write the failing CLI test**

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, registryDoc, manifestDoc, gitmodules string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "projects", "example", ".harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".harness", "registry.yaml"), []byte(registryDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "example", ".harness", "project.yaml"), []byte(manifestDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitmodules"), []byte(gitmodules), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestValidateAndList(t *testing.T) {
	root := writeFixture(t, `schemaVersion: harness.registry/v1
projects:
  - id: example
    path: projects/example
    github:
      repository: owner/repo
    actors:
      operator: actor/operator
`, `schemaVersion: harness/v1
project:
  id: example
  repository: owner/repo
  defaultBranch: main
stack:
  adapter: go-service
quality:
  profile: standard
data:
  classification: internal
`, "[submodule \"projects/example\"]\n\tpath = projects/example\n\turl = git@github.com:owner/repo.git\n")

	var out, errOut bytes.Buffer
	if code := Run([]string{"validate", "--workspace", root}, &out, &errOut); code != 0 {
		t.Fatalf("validate exit = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "example") || !strings.Contains(out.String(), "OK") {
		t.Fatalf("unexpected validate output: %q", out.String())
	}

	out.Reset()
	if code := Run([]string{"list", "--workspace", root}, &out, &errOut); code != 0 {
		t.Fatalf("list exit = %d", code)
	}
	if !strings.Contains(out.String(), "example") || !strings.Contains(out.String(), "projects/example") {
		t.Fatalf("unexpected list output: %q", out.String())
	}
}

func TestValidateRejectsBadWorkspace(t *testing.T) {
	root := writeFixture(t, "schemaVersion: bogus\nprojects: []\n", "schemaVersion: harness/v1\n", "")

	var out, errOut bytes.Buffer
	if code := Run([]string{"validate", "--workspace", root}, &out, &errOut); code == 0 {
		t.Fatal("expected non-zero exit for bad workspace")
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/cli -run 'TestValidate' -count=1`

Expected: FAIL because package `cli` and `Run` are undefined.

- [ ] **Step 3: Implement the CLI with three subcommands**

```go
// internal/cli/cli.go
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/marceloamoreno87/workflow-dev-template/internal/registry"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: harness <validate|list|register> --workspace <dir>")
		return 2
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "register":
		return runRegister(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func workspaceFlag(fs *flag.FlagSet) *string {
	return fs.String("workspace", ".", "Harness Workspace root")
}

func loadWorkspace(root string) (registry.Registry, map[string]registry.ProjectManifest, string, error) {
	regData, err := os.ReadFile(filepath.Join(root, ".harness", "registry.yaml"))
	if err != nil {
		return registry.Registry{}, nil, "", err
	}
	reg, err := registry.ParseRegistry(regData)
	if err != nil {
		return registry.Registry{}, nil, "", err
	}
	if err := reg.Validate(); err != nil {
		return registry.Registry{}, nil, "", err
	}
	manifests := map[string]registry.ProjectManifest{}
	for _, p := range reg.Projects {
		raw, err := os.ReadFile(filepath.Join(root, p.Path, ".harness", "project.yaml"))
		if err != nil {
			return registry.Registry{}, nil, "", err
		}
		if err := registry.RejectSecrets(raw); err != nil {
			return registry.Registry{}, nil, "", err
		}
		m, err := registry.ParseProjectManifest(raw)
		if err != nil {
			return registry.Registry{}, nil, "", err
		}
		if err := m.Validate(); err != nil {
			return registry.Registry{}, nil, "", err
		}
		manifests[p.ID] = m
	}
	modules, err := os.ReadFile(filepath.Join(root, ".gitmodules"))
	if err != nil {
		return registry.Registry{}, nil, "", err
	}
	if err := registry.CheckConsistency(reg, manifests, string(modules)); err != nil {
		return registry.Registry{}, nil, "", err
	}
	return reg, manifests, string(modules), nil
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := workspaceFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	reg, _, _, err := loadWorkspace(*root)
	if err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	for _, p := range reg.Projects {
		fmt.Fprintf(stdout, "OK %s %s\n", p.ID, p.Path)
	}
	return 0
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := workspaceFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	reg, _, _, err := loadWorkspace(*root)
	if err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	for _, p := range reg.Projects {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", p.ID, p.Path, p.Github.Repository)
	}
	return 0
}

func runRegister(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := workspaceFlag(fs)
	id := fs.String("id", "", "Project id")
	relPath := fs.String("path", "", "Project path below projects/")
	repo := fs.String("repository", "", "GitHub repository owner/name")
	operator := fs.String("operator", "actor/operator", "Operator actor")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == "" || *relPath == "" || *repo == "" {
		fmt.Fprintln(stderr, "register requires --id, --path, and --repository")
		return 2
	}
	regPath := filepath.Join(*root, ".harness", "registry.yaml")
	var reg registry.Registry
	if data, err := os.ReadFile(regPath); err == nil {
		var err error
		reg, err = registry.ParseRegistry(data)
		if err != nil {
			fmt.Fprintln(stderr, "invalid:", err)
			return 1
		}
	} else {
		reg = registry.Registry{SchemaVersion: "harness.registry/v1"}
	}
	entry := registry.RegistryProject{ID: *id, Path: *relPath}
	entry.Github.Repository = *repo
	entry.Actors.Operator = *operator
	reg.Projects = append(reg.Projects, entry)
	if err := reg.Validate(); err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	out, err := yaml.Marshal(reg)
	if err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Join(*root, ".harness"), 0o755); err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	if err := os.WriteFile(regPath, out, 0o644); err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	fmt.Fprintf(stdout, "registered %s\n", *id)
	return 0
}
```

```go
// cmd/harness/main.go
package main

import (
	"os"

	"github.com/marceloamoreno87/workflow-dev-template/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/cli -count=1 && go build ./...`

Expected: PASS and clean build.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/cli/cli.go internal/cli/cli_test.go cmd/harness/main.go && go test ./...`

Expected: PASS.

```bash
git add internal/cli/cli.go internal/cli/cli_test.go cmd/harness/main.go
git commit -m "feat(cli): register validate and list projects"
```

### Task 5: Prove registration round-trip and workspace safety

**Files:**
- Create: `internal/cli/roundtrip_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete `cli.Run` from Task 4 plus `registry` validation from Tasks 1-3.
- Produces: executable evidence that `register` then `validate` then `list` agree and invalid workspaces never register.

- [ ] **Step 1: Add the round-trip test**

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterValidateListRoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var out, errOut bytes.Buffer

	if code := Run([]string{"register", "--workspace", root, "--id", "demo", "--path", "projects/demo", "--repository", "owner/demo"}, &out, &errOut); code != 0 {
		t.Fatalf("register exit = %d, stderr = %q", code, errOut.String())
	}
	if err := os.MkdirAll(filepath.Join(root, "projects", "demo", ".harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "schemaVersion: harness/v1\nproject:\n  id: demo\n  repository: owner/demo\n  defaultBranch: main\nstack:\n  adapter: go-service\nquality:\n  profile: prototype\ndata:\n  classification: public\n"
	if err := os.WriteFile(filepath.Join(root, "projects", "demo", ".harness", "project.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitmodules"), []byte("[submodule \"projects/demo\"]\n\tpath = projects/demo\n\turl = git@github.com:owner/demo.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if code := Run([]string{"validate", "--workspace", root}, &out, &errOut); code != 0 {
		t.Fatalf("validate exit = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "OK demo") {
		t.Fatalf("unexpected validate output: %q", out.String())
	}

	out.Reset()
	if code := Run([]string{"list", "--workspace", root}, &out, &errOut); code != 0 {
		t.Fatalf("list exit = %d", code)
	}
	if !strings.Contains(out.String(), "demo\tprojects/demo\towner/demo") {
		t.Fatalf("unexpected list output: %q", out.String())
	}
}

func TestRegisterRejectsEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run([]string{"register", "--workspace", root, "--id", "evil", "--path", "projects/../evil", "--repository", "owner/evil"}, &out, &errOut); code == 0 {
		t.Fatal("expected non-zero exit for escaping path")
	}
	if _, err := os.Stat(filepath.Join(root, ".harness", "registry.yaml")); !os.IsNotExist(err) {
		t.Fatalf("escaping register must not persist a registry, stat err = %v", err)
	}
}
```

- [ ] **Step 2: Run the round-trip test**

Run: `go test ./internal/cli -run TestRegister -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean build of `cmd/harness`.

- [ ] **Step 4: Mark Increment 3 verified and commit**

In `docs/implementation-plan.md`, add a checklist below Increment 3 with the commands and evidence required to mark it complete. Do not mark it complete until the commands above pass in the implementation Project Worktree.

```bash
git add internal/cli/roundtrip_test.go docs/implementation-plan.md
git commit -m "test(cli): verify register validate list round-trip"
```
