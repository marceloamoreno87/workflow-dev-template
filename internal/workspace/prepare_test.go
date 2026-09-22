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
