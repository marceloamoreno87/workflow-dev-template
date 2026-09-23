package daemon

import (
	"strings"
	"testing"
)

func TestSystemdUnit(t *testing.T) {
	t.Parallel()

	unit, err := SystemdUnit("/usr/local/bin/harnessd", "/home/op/.harness/daemon.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Description=Harness daemon",
		"ExecStart=/usr/local/bin/harnessd --config /home/op/.harness/daemon.yaml",
		"WantedBy=default.target",
		"Restart=on-failure",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
	if _, err := SystemdUnit("", "/home/op/.harness/daemon.yaml"); err == nil {
		t.Fatal("expected empty exec rejection, got none")
	}
}
