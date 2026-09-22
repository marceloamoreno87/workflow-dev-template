package stack

import (
	"testing"
	"time"
)

func TestCatalogResolves(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"go-service", "python-service", "nextjs-web"} {
		adapter, err := Resolve(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := adapter.Validate(); err != nil {
			t.Fatalf("%s invalid: %v", name, err)
		}
	}
	if _, err := Resolve("cobol-mainframe"); err == nil {
		t.Fatal("expected unknown rejection, got none")
	}
}

func TestGoServiceDeclaresBaseline(t *testing.T) {
	t.Parallel()

	adapter, err := Resolve("go-service")
	if err != nil {
		t.Fatal(err)
	}
	for _, gate := range []string{"format", "lint", "test"} {
		if _, ok := adapter.Gates[gate]; !ok {
			t.Fatalf("go-service missing %q", gate)
		}
	}
}

func TestRejectBadAdapters(t *testing.T) {
	t.Parallel()

	base, err := Resolve("python-service")
	if err != nil {
		t.Fatal(err)
	}
	mk := func(mut func(*Adapter)) Adapter {
		adapter := base
		adapter.Gates = map[string]GateSpec{}
		for k, v := range base.Gates {
			adapter.Gates[k] = v
		}
		mut(&adapter)
		return adapter
	}
	for _, tc := range []struct {
		name    string
		adapter Adapter
	}{
		{name: "bad name", adapter: func() Adapter {
			a := mk(func(*Adapter) {})
			a.Name = "Bad Adapter!"
			return a
		}()},
		{name: "empty gates", adapter: mk(func(a *Adapter) { a.Gates = map[string]GateSpec{} })},
		{name: "bad gate", adapter: mk(func(a *Adapter) {
			a.Gates["teleport"] = GateSpec{Command: []string{"teleport"}, Timeout: time.Minute}
		})},
		{name: "absolute binary", adapter: mk(func(a *Adapter) {
			a.Gates["test"] = GateSpec{Command: []string{"/bin/evil", "x"}, Timeout: time.Minute}
		})},
		{name: "escape workdir", adapter: mk(func(a *Adapter) {
			a.Gates["test"] = GateSpec{Command: []string{"pytest", "-q"}, WorkdirRel: "../evil", Timeout: time.Minute}
		})},
		{name: "short timeout", adapter: mk(func(a *Adapter) {
			a.Gates["test"] = GateSpec{Command: []string{"pytest", "-q"}, Timeout: time.Second}
		})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.adapter.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
