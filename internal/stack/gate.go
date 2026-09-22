// internal/stack/gate.go
package stack

import (
	"fmt"
	"path/filepath"
	"strings"
)

func GateCommand(a Adapter, gate, projectDir string) ([]string, string, error) {
	if err := a.Validate(); err != nil {
		return nil, "", err
	}
	spec, ok := a.Gates[gate]
	if !ok {
		return nil, "", fmt.Errorf("%w: gate %q not declared by %q", ErrStack, gate, a.Name)
	}
	if !filepath.IsAbs(projectDir) {
		return nil, "", fmt.Errorf("%w: project dir must be absolute", ErrStack)
	}
	cwd := filepath.Clean(projectDir)
	if spec.WorkdirRel != "" {
		cwd = filepath.Join(cwd, spec.WorkdirRel)
		rel, err := filepath.Rel(filepath.Clean(projectDir), cwd)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, "", fmt.Errorf("%w: gate workdir escapes project", ErrStack)
		}
	}
	return append([]string{}, spec.Command...), cwd, nil
}
