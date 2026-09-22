package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunTruncatesOutput(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	withStubDocker(t, "spam")

	task := validTask()
	res, err := Run(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated {
		t.Fatal("expected truncation flag, got none")
	}
	if len(res.Stdout) > outputCap {
		t.Fatalf("stdout exceeds cap: %d", len(res.Stdout))
	}
	if res.Image != task.Image || len(res.Command) != len(task.Command) {
		t.Fatalf("result lost command identity: %#v", res)
	}
}

func TestRunRealDocker(t *testing.T) {
	if os.Getenv("HARNESS_RUNNER_DOCKER") == "" {
		t.Skip("set HARNESS_RUNNER_DOCKER=1 to run against a real daemon")
	}
	out, err := exec.Command("docker", "inspect", "--format", "{{.RepoDigests}}", "pgvector/pgvector:pg16").CombinedOutput()
	if err != nil {
		t.Skipf("fixture image unavailable: %v", err)
	}
	digest := strings.Trim(strings.TrimSpace(string(out)), "[]")
	if !strings.Contains(digest, "@sha256:") {
		t.Skipf("no digest for fixture image: %q", digest)
	}
	task := validTask()
	task.Image = digest
	task.Command = []string{"/bin/echo", "harness-probe"}
	task.Timeout = 2 * time.Minute
	root := t.TempDir()
	workdir := filepath.Join(root, ".worktrees", "demo")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	task.WorkspaceRoot = root
	task.Workdir = workdir
	res, err := Run(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || res.Stdout != "harness-probe\n" {
		t.Fatalf("unexpected real result: %#v", res)
	}
}
