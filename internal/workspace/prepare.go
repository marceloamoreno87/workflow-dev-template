// internal/workspace/prepare.go
package workspace

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func baseEnv() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_TERMINAL_PROMPT=0",
	}
	for _, key := range []string{"LANG", "LC_ALL", "TZ"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func (p Prepared) Env() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_GLOBAL=" + p.ConfigFile,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ALLOW_PROTOCOL=https:ssh:",
	}
	for _, key := range []string{"LANG", "LC_ALL", "TZ"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func runGit(dir string, env []string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := out.String()
		if len(msg) > 2048 {
			msg = msg[:2048]
		}
		return fmt.Errorf("%w: git %s: %v: %s", ErrWorkspace, strings.Join(args, " "), err, msg)
	}
	return nil
}

func Prepare(root, projectID, submodulePath string) (Prepared, error) {
	if err := ValidateRequest(root, projectID, submodulePath); err != nil {
		return Prepared{}, err
	}
	dir, err := WorktreeDir(root, projectID)
	if err != nil {
		return Prepared{}, err
	}
	if _, err := os.Stat(dir); err == nil {
		return Prepared{}, fmt.Errorf("%w: worktree dir %q exists, close it first", ErrWorkspace, dir)
	}
	if err := os.MkdirAll(filepath.Join(root, ".worktrees"), 0o755); err != nil {
		return Prepared{}, err
	}
	if err := runGit(submodulePath, baseEnv(), "worktree", "add", "--detach", dir, "HEAD"); err != nil {
		return Prepared{}, err
	}
	prepared, err := WriteGitConfig(dir)
	if err != nil {
		_ = runGit(submodulePath, baseEnv(), "worktree", "remove", "--force", dir)
		return Prepared{}, err
	}
	return prepared, nil
}

func Close(root, projectID, submodulePath string) error {
	if err := ValidateRequest(root, projectID, submodulePath); err != nil {
		return err
	}
	dir, err := WorktreeDir(root, projectID)
	if err != nil {
		return err
	}
	if err := runGit(submodulePath, baseEnv(), "worktree", "remove", "--force", dir); err != nil {
		return err
	}
	_ = runGit(submodulePath, baseEnv(), "worktree", "prune")
	if _, err := os.Stat(dir); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}
