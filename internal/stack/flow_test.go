package stack

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestAdapterFlow(t *testing.T) {
	for _, tc := range []struct {
		adapter string
		binary  string
		argv    []string
		fixture string
		name    string
		want    string
	}{
		{adapter: "python-service", binary: "python3", argv: []string{"python3", "-m", "py_compile", "fixture_mod.py"}, fixture: "VALUE = 1\n", name: "fixture_mod.py", want: ""},
		{adapter: "nextjs-web", binary: "node", argv: []string{"node", "fixture.js"}, fixture: "console.log('flow-ok');\n", name: "fixture.js", want: "flow-ok\n"},
		{adapter: "go-service", binary: "gofmt", argv: []string{"gofmt", "-l", "."}, fixture: "package fixture\n", name: "fixture.go", want: ""},
	} {
		t.Run(tc.adapter, func(t *testing.T) {
			if _, err := exec.LookPath(tc.binary); err != nil {
				t.Skipf("%s not installed", tc.binary)
			}
			resolved, err := Resolve(tc.adapter)
			if err != nil {
				t.Fatal(err)
			}
			if err := resolved.Validate(); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.name), []byte(tc.fixture), 0o644); err != nil {
				t.Fatal(err)
			}
			res, err := Run(context.Background(), tc.argv, dir, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if res.Output != tc.want {
				t.Fatalf("unexpected output: %#v", res)
			}
		})
	}
}
