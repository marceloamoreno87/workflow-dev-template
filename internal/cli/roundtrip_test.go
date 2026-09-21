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
