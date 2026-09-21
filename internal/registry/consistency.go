// internal/registry/consistency.go
package registry

import (
	"fmt"
	"strings"
)

func CheckConsistency(reg Registry, manifests map[string]ProjectManifest, gitmodules string) error {
	for _, p := range reg.Projects {
		m, ok := manifests[p.ID]
		if !ok {
			return fmt.Errorf("%w: missing manifest for project %q", ErrManifest, p.ID)
		}
		if m.Project.ID != p.ID {
			return fmt.Errorf("%w: manifest id %q disagrees with registry %q", ErrManifest, m.Project.ID, p.ID)
		}
		if m.Project.Repository != p.Github.Repository {
			return fmt.Errorf("%w: manifest repository %q disagrees with registry %q", ErrManifest, m.Project.Repository, p.Github.Repository)
		}
		needle := "path = " + p.Path
		found := false
		for _, line := range strings.Split(gitmodules, "\n") {
			if strings.TrimSpace(line) == needle {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: path %q missing from submodule record", ErrManifest, p.Path)
		}
	}
	return nil
}
