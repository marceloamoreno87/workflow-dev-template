// internal/registry/project.go
package registry

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrSchemaVersion = errors.New("unsupported schema version")
	ErrManifest      = errors.New("invalid project manifest")
	ErrSecret        = errors.New("secret material in manifest")
)

type ProjectManifest struct {
	SchemaVersion string `yaml:"schemaVersion"`
	Project       struct {
		ID            string `yaml:"id"`
		Repository    string `yaml:"repository"`
		DefaultBranch string `yaml:"defaultBranch"`
	} `yaml:"project"`
	Stack struct {
		Adapter string `yaml:"adapter"`
	} `yaml:"stack"`
	Quality struct {
		Profile string `yaml:"profile"`
	} `yaml:"quality"`
	Data struct {
		Classification string `yaml:"classification"`
	} `yaml:"data"`
}

func ParseProjectManifest(data []byte) (ProjectManifest, error) {
	var m ProjectManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return ProjectManifest{}, err
	}
	if m.SchemaVersion != "harness/v1" {
		return ProjectManifest{}, fmt.Errorf("%w: %q", ErrSchemaVersion, m.SchemaVersion)
	}
	return m, nil
}

func (m ProjectManifest) Validate() error {
	if m.Project.ID == "" || m.Project.Repository == "" {
		return fmt.Errorf("%w: project id and repository are required", ErrManifest)
	}
	if m.Stack.Adapter == "" {
		return fmt.Errorf("%w: stack adapter is required", ErrManifest)
	}
	switch m.Quality.Profile {
	case "prototype", "standard", "critical":
	default:
		return fmt.Errorf("%w: quality profile %q", ErrManifest, m.Quality.Profile)
	}
	switch m.Data.Classification {
	case "public", "internal", "confidential", "restricted":
	default:
		return fmt.Errorf("%w: data classification %q", ErrManifest, m.Data.Classification)
	}
	return nil
}

var secretKeys = map[string]bool{
	"password": true, "secret": true, "token": true, "apikey": true,
	"api_key": true, "privatekey": true, "private_key": true, "credentials": true,
}

func RejectSecrets(data []byte) error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	return rejectSecretsMap(raw)
}

func rejectSecretsMap(m map[string]any) error {
	for k, v := range m {
		flat := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(k, "_", ""), "-", ""))
		if secretKeys[flat] {
			return fmt.Errorf("%w: key %q", ErrSecret, k)
		}
		if nested, ok := v.(map[string]any); ok {
			if err := rejectSecretsMap(nested); err != nil {
				return err
			}
		}
	}
	return nil
}
