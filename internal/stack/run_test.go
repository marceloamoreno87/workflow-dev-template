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
