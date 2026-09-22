package workspace

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPrepareSanitizesEnvironment(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	t.Setenv("GH_TOKEN", "host-secret")
	t.Setenv("GITHUB_TOKEN", "host-secret")
	t.Setenv("COOLIFY_TOKEN", "host-secret")
	t.Setenv("TELEGRAM_BOT_TOKEN", "host-secret")
	t.Setenv("MY_SECRET_VALUE", "host-secret")
	t.Setenv("DB_PASSWORD", "host-secret")

	root, submodule := fixtureWorkspace(t)
	prepared, err := Prepare(root, "demo", submodule)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Close(root, "demo", submodule); err != nil {
			t.Fatal(err)
		}
	})
	for _, entry := range prepared.Env() {
		upper := strings.ToUpper(entry)
		if strings.Contains(entry, "host-secret") {
			t.Fatalf("secret value leaked into env: %q", entry)
		}
		for _, banned := range []string{"GH_TOKEN", "GITHUB_TOKEN", "COOLIFY_TOKEN", "TELEGRAM_BOT_TOKEN", "MY_SECRET_VALUE", "DB_PASSWORD"} {
			if strings.HasPrefix(upper, banned+"=") {
				t.Fatalf("secret variable leaked into env: %q", entry)
			}
		}
	}
	joined := strings.Join(prepared.Env(), "\n")
	for _, want := range []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_ALLOW_PROTOCOL=https:ssh:"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("hardening %q missing from env:\n%s", want, joined)
		}
	}
}

func TestPreparedHooksAndHelpersAreDisabled(t *testing.T) {
	t.Parallel()

	root, submodule := fixtureWorkspace(t)
	prepared, err := Prepare(root, "demo", submodule)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Close(root, "demo", submodule); err != nil {
			t.Fatal(err)
		}
	})
	cmd := exec.Command("git", "config", "--list")
	cmd.Dir = prepared.Dir
	cmd.Env = prepared.Env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git config --list: %v\n%s", err, out)
	}
	list := string(out)
	if !strings.Contains(list, "core.hookspath="+prepared.HooksDir) {
		t.Fatalf("hooksPath not enforced:\n%s", list)
	}
	for _, line := range strings.Split(list, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "credential.helper") && len(strings.TrimSpace(strings.TrimPrefix(lower, "credential.helper="))) > 0 {
			t.Fatalf("credential helper survives: %q", line)
		}
	}
}
