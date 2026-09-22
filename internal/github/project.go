// internal/github/project.go
package github

import (
	"errors"
	"fmt"
	"strings"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

var ErrProjectStatus = errors.New("unknown project status")

func ParseProjectStatus(s string) (workflow.State, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", nil
	}
	flat := strings.ToLower(strings.NewReplacer(" ", "_", "-", "_").Replace(trimmed))
	collapsed := strings.Builder{}
	prev := false
	for _, r := range flat {
		if r == '_' {
			if !prev {
				collapsed.WriteRune(r)
			}
			prev = true
			continue
		}
		prev = false
		collapsed.WriteRune(r)
	}
	state := workflow.State(collapsed.String())
	if !state.Valid() {
		return "", fmt.Errorf("%w: %q", ErrProjectStatus, s)
	}
	return state, nil
}
