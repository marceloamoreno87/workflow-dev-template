package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// fixtureWorkspace builds an outer git repo with a real submodule checkout at
// projects/demo plus registry and manifest, returning the workspace root.
func fixtureWorkspace(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	src := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, src, "init", "-b", "main")
	gitRun(t, src, "config", "user.name", "fixture")
	gitRun(t, src, "config", "user.email", "fixture@localhost")
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, src, "add", "README.md")
	gitRun(t, src, "commit", "-m", "initial")

	gitRun(t, root, "init", "-b", "main")
	gitRun(t, root, "config", "user.name", "fixture")
	gitRun(t, root, "config", "user.email", "fixture@localhost")
	gitRun(t, root, "-c", "protocol.file.allow=always", "submodule", "add", src, "projects/demo")
	gitRun(t, root, "commit", "-m", "link demo")

	registry := `schemaVersion: harness.registry/v1
projects:
  - id: demo
    path: projects/demo
    github:
      repository: owner/repo
    actors:
      operator: actor/operator
`
	if err := os.MkdirAll(filepath.Join(root, ".harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".harness", "registry.yaml"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: harness/v1
project:
  id: demo
  repository: owner/repo
stack:
  adapter: go-service
quality:
  profile: standard
data:
  classification: internal
`
	sub := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(filepath.Join(sub, ".harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, ".harness", "project.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeStub(t *testing.T, bin, name, script string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

const stubCodexScript = `#!/bin/sh
stub_dir=$(dirname "$0")
echo "$@" >> "$stub_dir/argv.log"
cat > "$stub_dir/stdin.txt"
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; fi
  prev="$arg"
done
case "$(cat "$stub_dir/mode")" in
  ok)
    cat "$stub_dir/events.jsonl"
    cp "$stub_dir/final.json" "$out"
    ;;
  blocked)
    echo '{"type":"thread.started","thread_id":"thread-9"}'
    echo '{"status":"blocked","summary":"needs input","changes":[],"evidenceRefs":[],"blockers":["waiting"],"usage":{"inputTokens":5,"outputTokens":2},"knowledgeProposals":[],"privilegedRequests":[]}' > "$out"
    ;;
esac
exit 0
`

const stubCodexEvents = `{"type":"thread.started","thread_id":"thread-1"}
{"type":"item.completed"}
`

const stubCodexFinal = `{"status":"completed","summary":"done","changes":["a"],"evidenceRefs":[],"blockers":[],"usage":{"inputTokens":10,"outputTokens":5},"knowledgeProposals":[],"privilegedRequests":[]}`

// stubBin creates a bin dir with a mode-driven stub codex plus codex fixture files,
// prepends it to PATH, and points CODEX_HOME at a temp dir. No t.Parallel in callers.
func stubBin(t *testing.T) string {
	t.Helper()

	bin := t.TempDir()
	writeStub(t, bin, "codex", stubCodexScript)
	if err := os.WriteFile(filepath.Join(bin, "mode"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "events.jsonl"), []byte(stubCodexEvents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "final.json"), []byte(stubCodexFinal), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CODEX_HOME", t.TempDir())
	return bin
}
