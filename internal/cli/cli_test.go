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
