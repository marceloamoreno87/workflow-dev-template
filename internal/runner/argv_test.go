package runner

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestArgvIsExact(t *testing.T) {
	t.Parallel()

	task := validTask()
	got, err := Argv(task, "harness-test", "/tmp/test.cid")
	if err != nil {
		t.Fatal(err)
	}
	user := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	want := []string{
		"run", "--rm", "--name", "harness-test", "--cidfile", "/tmp/test.cid",
		"--network", "none",
		"--user", user,
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--pids-limit", "256",
		"--memory", "512m", "--memory-swap", "512m",
		"--cpus", "2",
		"--read-only", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m",
		"--mount", "type=bind,src=/ws/.worktrees/demo,dst=/work,rw",
		"--workdir", "/work",
		"--entrypoint", "/bin/echo",
		"--env", "GOFLAGS=-count=1",
		task.Image, "hi",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch:\n got: %q\nwant: %q", got, want)
	}
	for _, arg := range got {
		if arg == "--privileged" || strings.HasPrefix(arg, "--volume=") || arg == "-v" {
			t.Fatalf("forbidden flag in argv: %q", arg)
		}
	}
}

func TestArgvRejectsInvalidTask(t *testing.T) {
	t.Parallel()

	task := validTask()
	task.Image = "example/gate:latest"
	if _, err := Argv(task, "harness-test", "/tmp/test.cid"); err == nil {
		t.Fatal("expected rejection, got none")
	}
}
