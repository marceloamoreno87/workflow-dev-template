// internal/registry/registry.go
package registry

import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type Registry struct {
	SchemaVersion string            `yaml:"schemaVersion"`
	Projects      []RegistryProject `yaml:"projects"`
}

type RegistryProject struct {
	ID     string `yaml:"id"`
	Path   string `yaml:"path"`
	Github struct {
		Repository       string `yaml:"repository"`
		ClientProject    string `yaml:"clientProject"`
		PortfolioProject string `yaml:"portfolioProject"`
	} `yaml:"github"`
	Actors struct {
		Operator string   `yaml:"operator"`
		Clients  []string `yaml:"clients"`
	} `yaml:"actors"`
}

func ParseRegistry(data []byte) (Registry, error) {
	var r Registry
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Registry{}, err
	}
	if r.SchemaVersion != "harness.registry/v1" {
		return Registry{}, fmt.Errorf("%w: %q", ErrSchemaVersion, r.SchemaVersion)
	}
	return r, nil
}

func (r Registry) Validate() error {
	ids := map[string]bool{}
	paths := map[string]bool{}
	for _, p := range r.Projects {
		if p.ID == "" {
			return fmt.Errorf("%w: project id is required", ErrManifest)
		}
		if ids[p.ID] {
			return fmt.Errorf("%w: duplicate project id %q", ErrManifest, p.ID)
		}
		ids[p.ID] = true
		clean := path.Clean(p.Path)
		if path.IsAbs(p.Path) || clean != p.Path || !strings.HasPrefix(clean, "projects/") || clean == "projects/" || strings.Contains(clean, "..") {
			return fmt.Errorf("%w: path %q must resolve below projects/", ErrManifest, p.Path)
		}
		if paths[clean] {
			return fmt.Errorf("%w: duplicate project path %q", ErrManifest, p.Path)
		}
		paths[clean] = true
		if p.Github.Repository == "" {
			return fmt.Errorf("%w: github repository is required for %q", ErrManifest, p.ID)
		}
		if p.Actors.Operator == "" {
			return fmt.Errorf("%w: operator actor is required for %q", ErrManifest, p.ID)
		}
	}
	return nil
}
