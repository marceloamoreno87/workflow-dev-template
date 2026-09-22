package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const stubCodex = `#!/bin/sh
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
  ok) cat "$stub_dir/events.jsonl"; cp "$stub_dir/final.json" "$out" ;;
  badfinal) echo "[]"; echo "not the schema" > "$out" ;;
  badline) echo "this is not json"; echo '{"status":"completed","summary":"x"}' > "$out" ;;
  slow) exec sleep 60 ;;
esac
exit 0
`

const stubEvents = `{"type":"thread.started","thread_id":"thread-1"}
{"type":"item.completed"}
`

const stubFinal = `{"status":"completed","summary":"done","changes":["a"],"evidenceRefs":[],"blockers":[],"usage":{"inputTokens":10,"outputTokens":5},"knowledgeProposals":[],"privilegedRequests":[]}`

func withStubCodex(t *testing.T, mode string) (bin string) {
	t.Helper()

	bin = t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(stubCodex), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "mode"), []byte(mode), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "events.jsonl"), []byte(stubEvents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "final.json"), []byte(stubFinal), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CODEX_HOME", t.TempDir())
	return bin
}

func TestRunThread(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	bin := withStubCodex(t, "ok")

	spec := validSpec()
	spec.Deadline = time.Now().Add(time.Hour)
	res, err := Run(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "completed" || res.Summary != "done" || res.ThreadID != "thread-1" || res.TranscriptEvents != 2 {
		t.Fatalf("unexpected result: %#v", res)
	}
	if res.Usage.InputTokens != 10 || res.Usage.OutputTokens != 5 {
		t.Fatalf("usage lost: %#v", res.Usage)
	}
	raw, err := os.ReadFile(filepath.Join(bin, "argv.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--sandbox workspace-write", "--output-schema", "--json", "-"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("argv missing %q:\n%s", want, raw)
		}
	}
	prompt, err := os.ReadFile(filepath.Join(bin, "stdin.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(prompt) != Prompt(spec) {
		t.Fatalf("prompt mismatch:\n%s", prompt)
	}
}

func TestRunRejectsBadOutput(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	withStubCodex(t, "badfinal")

	if _, err := Run(context.Background(), validSpec()); err == nil {
		t.Fatal("expected final-shape rejection, got none")
	}
	withStubCodex(t, "badline")
	if _, err := Run(context.Background(), validSpec()); err == nil {
		t.Fatal("expected transcript rejection, got none")
	}
}

func TestRunTimeout(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	withStubCodex(t, "slow")

	spec := validSpec()
	spec.Timeout = time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	res, err := Run(ctx, spec)
	if err == nil {
		t.Fatal("expected timeout, got none")
	}
	if !res.TimedOut {
		t.Fatalf("timeout not flagged: %#v", res)
	}
}

func TestRunRejectsExpiredDeadline(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	withStubCodex(t, "ok")

	spec := validSpec()
	spec.Deadline = time.Now().Add(-time.Hour)
	if _, err := Run(context.Background(), spec); err == nil {
		t.Fatal("expected deadline rejection, got none")
	}
}
