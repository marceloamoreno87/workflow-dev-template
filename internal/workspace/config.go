// internal/workspace/config.go
package workspace

import (
	"os"
	"path/filepath"
)

type Prepared struct {
	Dir        string
	ConfigFile string
	HooksDir   string
}

func WriteGitConfig(worktreeDir string) (Prepared, error) {
	abs, err := cleanAbs(worktreeDir)
	if err != nil {
		return Prepared{}, err
	}
	gitDir := filepath.Join(abs, ".harness-git")
	hooks := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		return Prepared{}, err
	}
	configFile := filepath.Join(gitDir, "config")
	cfg := "[user]\n\tname = harness\n\temail = harness@localhost\n[core]\n\thooksPath = " + hooks + "\n[credential]\n\thelper = \n"
	if err := os.WriteFile(configFile, []byte(cfg), 0o644); err != nil {
		return Prepared{}, err
	}
	return Prepared{Dir: abs, ConfigFile: configFile, HooksDir: hooks}, nil
}
