package codex

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestArgvIsExact(t *testing.T) {
	t.Parallel()

	spec := validSpec()
	got, err := Argv(spec, "/tmp/schema.json", "/tmp/out.json")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"exec", "--json", "--sandbox", "workspace-write", "--ignore-user-config",
		"-C", "/ws/.worktrees/demo",
		"--add-dir", "/ws/.worktrees/demo/.tmp-task",
		"-m", "gpt-5.6-terra", "-c", `model_reasoning_effort="medium"`,
		"--output-schema", "/tmp/schema.json", "-o", "/tmp/out.json",
		"-",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch:\n got: %q\nwant: %q", got, want)
	}
	for _, arg := range got {
		if arg == "--dangerously-bypass-approvals-and-sandbox" || arg == "danger-full-access" || arg == "--worktree" {
			t.Fatalf("forbidden flag in argv: %q", arg)
		}
	}
}

func TestArgvRejectsInvalidSpec(t *testing.T) {
	t.Parallel()

	spec := validSpec()
	spec.Classification = "restricted"
	if _, err := Argv(spec, "/tmp/schema.json", "/tmp/out.json"); err == nil {
		t.Fatal("expected rejection, got none")
	}
}

func TestEnvCarriesAuthAndGitConfig(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	t.Setenv("CODEX_HOME", "/tmp/codex-home")
	t.Setenv("GH_TOKEN", "host-secret")

	spec := validSpec()
	env, err := Env(spec)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "\n")
	for _, want := range []string{"CODEX_HOME=/tmp/codex-home", "GIT_CONFIG_GLOBAL=/ws/.worktrees/demo/.harness-git/config", "GIT_CONFIG_NOSYSTEM=1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("env missing %q:\n%s", want, joined)
		}
	}
	for _, entry := range env {
		if strings.Contains(entry, "host-secret") {
			t.Fatalf("secret leaked into env: %q", entry)
		}
	}
}

func TestEnvRejectsMissingHome(t *testing.T) {
	// No t.Parallel: CODEX_HOME mutation forbids parallel tests.
	prior, had := os.LookupEnv("CODEX_HOME")
	if err := os.Unsetenv("CODEX_HOME"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("CODEX_HOME", prior)
		}
	})
	if _, err := Env(validSpec()); err == nil {
		t.Fatal("expected missing CODEX_HOME rejection, got none")
	}
}
