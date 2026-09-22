package stack

import (
	"reflect"
	"testing"
	"time"
)

func TestGateCommandRenders(t *testing.T) {
	t.Parallel()

	adapter, err := Resolve("python-service")
	if err != nil {
		t.Fatal(err)
	}
	argv, cwd, err := GateCommand(adapter, "test", "/ws/projects/demo")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(argv, []string{"python", "-m", "pytest", "-q"}) {
		t.Fatalf("unexpected argv: %q", argv)
	}
	if cwd != "/ws/projects/demo" {
		t.Fatalf("unexpected cwd: %q", cwd)
	}
	sub, err := Resolve("nextjs-web")
	if err != nil {
		t.Fatal(err)
	}
	argv, cwd, err = GateCommand(sub, "build", "/ws/projects/web")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(argv, []string{"npm", "run", "build"}) || cwd != "/ws/projects/web" {
		t.Fatalf("unexpected render: %q %q", argv, cwd)
	}
}

func TestGateCommandRejects(t *testing.T) {
	t.Parallel()

	adapter, err := Resolve("go-service")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		gate       string
		projectDir string
	}{
		{name: "unknown gate", gate: "teleport", projectDir: "/ws/projects/demo"},
		{name: "relative dir", gate: "test", projectDir: "ws/projects/demo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := GateCommand(adapter, tc.gate, tc.projectDir); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
	evil := adapter
	evil.Gates["test"] = GateSpec{Command: []string{"go", "test"}, WorkdirRel: "../evil", Timeout: 60000000000}
	if _, _, err := GateCommand(evil, "test", "/ws/projects/demo"); err == nil {
		t.Fatal("expected workdir rejection, got none")
	}
}

func TestGateCommandResolvesSubdir(t *testing.T) {
	t.Parallel()

	adapter, err := Resolve("go-service")
	if err != nil {
		t.Fatal(err)
	}
	adapter.Gates["test"] = GateSpec{Command: []string{"go", "test", "./..."}, WorkdirRel: "backend", Timeout: time.Minute}
	_, cwd, err := GateCommand(adapter, "test", "/ws/projects/demo")
	if err != nil {
		t.Fatal(err)
	}
	if cwd != "/ws/projects/demo/backend" {
		t.Fatalf("unexpected cwd: %q", cwd)
	}
}
