// internal/cli/cli.go
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/marceloamoreno87/workflow-dev-template/internal/registry"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: harness <validate|list|register> --workspace <dir>")
		return 2
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "register":
		return runRegister(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func workspaceFlag(fs *flag.FlagSet) *string {
	return fs.String("workspace", ".", "Harness Workspace root")
}

func loadWorkspace(root string) (registry.Registry, map[string]registry.ProjectManifest, string, error) {
	regData, err := os.ReadFile(filepath.Join(root, ".harness", "registry.yaml"))
	if err != nil {
		return registry.Registry{}, nil, "", err
	}
	reg, err := registry.ParseRegistry(regData)
	if err != nil {
		return registry.Registry{}, nil, "", err
	}
	if err := reg.Validate(); err != nil {
		return registry.Registry{}, nil, "", err
	}
	manifests := map[string]registry.ProjectManifest{}
	for _, p := range reg.Projects {
		raw, err := os.ReadFile(filepath.Join(root, p.Path, ".harness", "project.yaml"))
		if err != nil {
			return registry.Registry{}, nil, "", err
		}
		if err := registry.RejectSecrets(raw); err != nil {
			return registry.Registry{}, nil, "", err
		}
		m, err := registry.ParseProjectManifest(raw)
		if err != nil {
			return registry.Registry{}, nil, "", err
		}
		if err := m.Validate(); err != nil {
			return registry.Registry{}, nil, "", err
		}
		manifests[p.ID] = m
	}
	modules, err := os.ReadFile(filepath.Join(root, ".gitmodules"))
	if err != nil {
		return registry.Registry{}, nil, "", err
	}
	if err := registry.CheckConsistency(reg, manifests, string(modules)); err != nil {
		return registry.Registry{}, nil, "", err
	}
	return reg, manifests, string(modules), nil
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := workspaceFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	reg, _, _, err := loadWorkspace(*root)
	if err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	for _, p := range reg.Projects {
		fmt.Fprintf(stdout, "OK %s %s\n", p.ID, p.Path)
	}
	return 0
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := workspaceFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	reg, _, _, err := loadWorkspace(*root)
	if err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	for _, p := range reg.Projects {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", p.ID, p.Path, p.Github.Repository)
	}
	return 0
}

func runRegister(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := workspaceFlag(fs)
	id := fs.String("id", "", "Project id")
	relPath := fs.String("path", "", "Project path below projects/")
	repo := fs.String("repository", "", "GitHub repository owner/name")
	operator := fs.String("operator", "actor/operator", "Operator actor")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == "" || *relPath == "" || *repo == "" {
		fmt.Fprintln(stderr, "register requires --id, --path, and --repository")
		return 2
	}
	regPath := filepath.Join(*root, ".harness", "registry.yaml")
	var reg registry.Registry
	if data, err := os.ReadFile(regPath); err == nil {
		var err error
		reg, err = registry.ParseRegistry(data)
		if err != nil {
			fmt.Fprintln(stderr, "invalid:", err)
			return 1
		}
	} else {
		reg = registry.Registry{SchemaVersion: "harness.registry/v1"}
	}
	entry := registry.RegistryProject{ID: *id, Path: *relPath}
	entry.Github.Repository = *repo
	entry.Actors.Operator = *operator
	reg.Projects = append(reg.Projects, entry)
	if err := reg.Validate(); err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	out, err := yaml.Marshal(reg)
	if err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Join(*root, ".harness"), 0o755); err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	if err := os.WriteFile(regPath, out, 0o644); err != nil {
		fmt.Fprintln(stderr, "invalid:", err)
		return 1
	}
	fmt.Fprintf(stdout, "registered %s\n", *id)
	return 0
}
