// internal/stack/adapter.go
package stack

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ErrStack = errors.New("invalid stack adapter")

var adapterPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

var gateNames = map[string]bool{
	"format": true, "lint": true, "typecheck": true,
	"test": true, "build": true, "smoke": true,
}

type GateSpec struct {
	Command    []string
	WorkdirRel string
	Timeout    time.Duration
}

type Adapter struct {
	Name    string
	Runtime string
	Gates   map[string]GateSpec
}

func (a Adapter) Validate() error {
	if !adapterPattern.MatchString(a.Name) {
		return fmt.Errorf("%w: adapter name %q", ErrStack, a.Name)
	}
	if a.Runtime == "" {
		return fmt.Errorf("%w: runtime required", ErrStack)
	}
	if len(a.Gates) == 0 {
		return fmt.Errorf("%w: at least one gate", ErrStack)
	}
	for name, gate := range a.Gates {
		if !gateNames[name] {
			return fmt.Errorf("%w: gate %q", ErrStack, name)
		}
		if err := gate.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (g GateSpec) Validate() error {
	if len(g.Command) == 0 || len(g.Command) > 8 {
		return fmt.Errorf("%w: command needs 1..8 args", ErrStack)
	}
	binary := g.Command[0]
	if binary == "" || strings.ContainsAny(binary, " \t\n\r/") {
		return fmt.Errorf("%w: binary must be a bare name", ErrStack)
	}
	for _, arg := range g.Command[1:] {
		if len([]rune(arg)) > 4096 {
			return fmt.Errorf("%w: arg too long", ErrStack)
		}
	}
	if g.WorkdirRel != "" {
		if filepath.IsAbs(g.WorkdirRel) {
			return fmt.Errorf("%w: workdir must be relative", ErrStack)
		}
		clean := filepath.Clean(g.WorkdirRel)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%w: workdir escapes", ErrStack)
		}
	}
	if g.Timeout < 10*time.Second || g.Timeout > 30*time.Minute {
		return fmt.Errorf("%w: timeout outside 10s..30m", ErrStack)
	}
	return nil
}

func Catalog() map[string]Adapter {
	return map[string]Adapter{
		"go-service": {
			Name:    "go-service",
			Runtime: "go",
			Gates: map[string]GateSpec{
				"format": {Command: []string{"gofmt", "-l", "."}, Timeout: time.Minute},
				"lint":   {Command: []string{"go", "vet", "./..."}, Timeout: 5 * time.Minute},
				"test":   {Command: []string{"go", "test", "./..."}, Timeout: 10 * time.Minute},
			},
		},
		"python-service": {
			Name:    "python-service",
			Runtime: "python",
			Gates: map[string]GateSpec{
				"format":    {Command: []string{"ruff", "format", "--check", "."}, Timeout: 2 * time.Minute},
				"lint":      {Command: []string{"ruff", "check", "."}, Timeout: 2 * time.Minute},
				"typecheck": {Command: []string{"mypy", "."}, Timeout: 5 * time.Minute},
				"test":      {Command: []string{"python", "-m", "pytest", "-q"}, Timeout: 10 * time.Minute},
			},
		},
		"nextjs-web": {
			Name:    "nextjs-web",
			Runtime: "node",
			Gates: map[string]GateSpec{
				"lint":      {Command: []string{"npm", "run", "lint"}, Timeout: 5 * time.Minute},
				"typecheck": {Command: []string{"npm", "run", "typecheck"}, Timeout: 5 * time.Minute},
				"test":      {Command: []string{"npm", "test"}, Timeout: 10 * time.Minute},
				"build":     {Command: []string{"npm", "run", "build"}, Timeout: 15 * time.Minute},
			},
		},
	}
}

func Resolve(name string) (Adapter, error) {
	adapter, ok := Catalog()[name]
	if !ok {
		return Adapter{}, fmt.Errorf("%w: unknown adapter %q", ErrStack, name)
	}
	return adapter, nil
}
