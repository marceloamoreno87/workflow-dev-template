// internal/runner/argv.go
package runner

import (
	"fmt"
	"os"
	"sort"
	"strconv"
)

func Argv(t Task, name, cidfile string) ([]string, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if name == "" || cidfile == "" {
		return nil, fmt.Errorf("%w: container name and cidfile are required", ErrTask)
	}
	argv := []string{
		"run", "--rm", "--name", name, "--cidfile", cidfile,
		"--network", "none",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--pids-limit", "256",
		"--memory", t.Memory, "--memory-swap", t.Memory,
		"--cpus", strconv.FormatFloat(t.CPUs, 'f', -1, 64),
		"--read-only", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m",
		"--mount", "type=bind,src=" + t.Workdir + ",dst=/work",
		"--workdir", "/work",
		"--entrypoint", t.Command[0],
	}
	keys := make([]string, 0, len(t.Env))
	for key := range t.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		argv = append(argv, "--env", key+"="+t.Env[key])
	}
	argv = append(argv, t.Image)
	argv = append(argv, t.Command[1:]...)
	return argv, nil
}
