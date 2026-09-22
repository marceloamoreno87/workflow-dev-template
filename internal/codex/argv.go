// internal/codex/argv.go
package codex

import (
	"fmt"
	"os"
)

func Argv(s ThreadSpec, schemaFile, outFile string) ([]string, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if schemaFile == "" || outFile == "" {
		return nil, fmt.Errorf("%w: schema file and output file are required", ErrThread)
	}
	argv := []string{
		"exec", "--json", "--sandbox", "workspace-write", "--ignore-user-config",
		"-C", s.Workdir,
	}
	for _, p := range s.WritablePaths {
		argv = append(argv, "--add-dir", p)
	}
	argv = append(argv,
		"-m", s.Model, "-c", fmt.Sprintf("model_reasoning_effort=%q", s.Effort),
		"--output-schema", schemaFile, "-o", outFile,
		"-",
	)
	return argv, nil
}

func Env(s ThreadSpec) ([]string, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		return nil, fmt.Errorf("%w: CODEX_HOME is required for local authentication", ErrThread)
	}
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"CODEX_HOME=" + home,
	}
	for _, key := range []string{"LANG", "LC_ALL", "TZ"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	if s.GitConfigGlobal != "" {
		env = append(env,
			"GIT_CONFIG_GLOBAL="+s.GitConfigGlobal,
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_TERMINAL_PROMPT=0",
		)
	}
	return env, nil
}
