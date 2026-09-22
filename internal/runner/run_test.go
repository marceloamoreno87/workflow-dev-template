package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const stubDocker = `#!/bin/sh
stub_dir=$(dirname "$0")
echo "$@" >> "$stub_dir/argv.log"
prev=""
for arg in "$@"; do
  if [ "$prev" = "--cidfile" ]; then echo "stub-cid" > "$arg"; fi
  prev="$arg"
done
case "$(cat "$stub_dir/mode")" in
  echo) echo "stub-hello" ;;
  fail) exit 3 ;;
  sleep) exec sleep 30 ;;
  spam)
    i=0
    while [ "$i" -lt 4000 ]; do echo "spam-line-$i-padding-padding-padding-padding"; i=$((i+1)); done
    ;;
esac
exit 0
`

func withStubDocker(t *testing.T, mode string) (log string) {
	t.Helper()

	bin := t.TempDir()
	script := filepath.Join(bin, "docker")
	if err := os.WriteFile(script, []byte(stubDocker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "mode"), []byte(mode), 0o644); err != nil {
		t.Fatal(err)
	}
	log = filepath.Join(bin, "argv.log")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func TestRunEcho(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	log := withStubDocker(t, "echo")

	task := validTask()
	res, err := Run(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || res.TimedOut || res.Stdout != "stub-hello\n" {
		t.Fatalf("unexpected result: %#v", res)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "--network none") || !strings.Contains(string(raw), task.Image) {
		t.Fatalf("argv not passed to docker binary:\n%s", raw)
	}
}

func TestRunFailureKeepsSecretsOutOfErrors(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	withStubDocker(t, "fail")

	task := validTask()
	task.Env = map[string]string{"APP_TOKEN": "s3cr3t-value"}
	_, err := Run(context.Background(), task)
	if err == nil {
		t.Fatal("expected failure, got none")
	}
	if strings.Contains(err.Error(), "s3cr3t-value") {
		t.Fatalf("secret leaked into error: %v", err)
	}
}

func TestRunTimeoutRemovesContainer(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	log := withStubDocker(t, "sleep")

	task := validTask()
	task.Timeout = 1100 * time.Millisecond
	start := time.Now()
	res, err := Run(context.Background(), task)
	if err == nil {
		t.Fatal("expected timeout, got none")
	}
	if !res.TimedOut || time.Since(start) > 20*time.Second {
		t.Fatalf("timeout not honored: %#v", res)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "rm -f") {
		t.Fatalf("timed-out container was not removed:\n%s", raw)
	}
}
