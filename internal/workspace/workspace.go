// internal/workspace/workspace.go
package workspace

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var ErrWorkspace = errors.New("invalid workspace request")

type Request struct {
	WorkspaceRoot string
	ProjectID     string
	SubmodulePath string
}

func validProjectID(id string) bool {
	if len(id) == 0 || len(id) > 100 {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func cleanAbs(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%w: path %q is not absolute", ErrWorkspace, p)
	}
	return filepath.Clean(p), nil
}

func WorktreeDir(root, projectID string) (string, error) {
	abs, err := cleanAbs(root)
	if err != nil {
		return "", err
	}
	if !validProjectID(projectID) {
		return "", fmt.Errorf("%w: project id %q", ErrWorkspace, projectID)
	}
	return filepath.Join(abs, ".worktrees", projectID), nil
}

func belowRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func ValidateRequest(root, projectID, submodulePath string) error {
	abs, err := cleanAbs(root)
	if err != nil {
		return err
	}
	if !validProjectID(projectID) {
		return fmt.Errorf("%w: project id %q", ErrWorkspace, projectID)
	}
	sub, err := cleanAbs(submodulePath)
	if err != nil {
		return err
	}
	if !belowRoot(abs, sub) || sub == abs {
		return fmt.Errorf("%w: submodule path %q escapes workspace", ErrWorkspace, submodulePath)
	}
	return nil
}
