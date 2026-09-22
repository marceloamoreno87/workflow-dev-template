// internal/runner/task.go
package runner

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrTask = errors.New("invalid runner task")

type Task struct {
	Image         string
	Command       []string
	WorkspaceRoot string
	Workdir       string
	Env           map[string]string
	Memory        string
	CPUs          float64
	Timeout       time.Duration
	Network       string
}

var imagePattern = regexp.MustCompile(`^([a-z0-9]+([._-][a-z0-9]+)*/)*[a-z0-9]+([._-][a-z0-9]+)*@sha256:[0-9a-f]{64}$`)

var memoryPattern = regexp.MustCompile(`^([1-9][0-9]*)(m|g)$`)

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (t Task) Validate() error {
	if !imagePattern.MatchString(t.Image) {
		return fmt.Errorf("%w: image must be digest-pinned", ErrTask)
	}
	if len(t.Command) == 0 || len(t.Command) > 32 {
		return fmt.Errorf("%w: command needs 1..32 args", ErrTask)
	}
	for _, arg := range t.Command {
		n := utf8.RuneCountInString(arg)
		if n == 0 || n > 4096 {
			return fmt.Errorf("%w: command arg length %d", ErrTask, n)
		}
	}
	if !strings.HasPrefix(t.Command[0], "/") {
		return fmt.Errorf("%w: command binary must be an absolute in-container path", ErrTask)
	}
	if !filepath.IsAbs(t.WorkspaceRoot) || !filepath.IsAbs(t.Workdir) {
		return fmt.Errorf("%w: workspace root and workdir must be absolute", ErrTask)
	}
	root := filepath.Clean(t.WorkspaceRoot)
	dir := filepath.Clean(t.Workdir)
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: workdir must resolve strictly below the workspace root", ErrTask)
	}
	m := memoryPattern.FindStringSubmatch(t.Memory)
	if m == nil {
		return fmt.Errorf("%w: memory must look like 512m or 2g", ErrTask)
	}
	var mb int
	fmt.Sscanf(m[1], "%d", &mb)
	if m[2] == "g" {
		mb *= 1024
	}
	if mb < 64 || mb > 8192 {
		return fmt.Errorf("%w: memory %dMB outside 64..8192MB", ErrTask, mb)
	}
	if !(t.CPUs > 0 && t.CPUs <= 8) {
		return fmt.Errorf("%w: cpus outside (0,8]", ErrTask)
	}
	if t.Timeout < time.Second || t.Timeout > 2*time.Hour {
		return fmt.Errorf("%w: timeout outside 1s..2h", ErrTask)
	}
	if t.Network != "none" {
		return fmt.Errorf("%w: network must be none", ErrTask)
	}
	if len(t.Env) > 64 {
		return fmt.Errorf("%w: too many env vars", ErrTask)
	}
	for key, value := range t.Env {
		if !envKeyPattern.MatchString(key) || len(key) > 128 {
			return fmt.Errorf("%w: env key %q", ErrTask, key)
		}
		if utf8.RuneCountInString(value) > 4096 {
			return fmt.Errorf("%w: env value for %q too long", ErrTask, key)
		}
	}
	return nil
}
