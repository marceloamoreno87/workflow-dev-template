package codex

import (
	"strings"
	"testing"
	"time"
)

func validSpec() ThreadSpec {
	return ThreadSpec{
		Role:               "implementer",
		Objective:          "Add the missing validation",
		SpecRef:            "owner/repo#123",
		Model:              "gpt-5.6-terra",
		Effort:             "medium",
		Classification:     "internal",
		WorkspaceRoot:      "/ws",
		Workdir:            "/ws/.worktrees/demo",
		WritablePaths:      []string{"/ws/.worktrees/demo/.tmp-task"},
		Skills:             []string{"harness/gates"},
		MCPServers:         []string{"harness"},
		BudgetUSD:          10,
		Deadline:           time.Now().Add(time.Hour),
		CompletionCriteria: []string{"go test ./... passes"},
		Timeout:            30 * time.Minute,
		GitConfigGlobal:    "/ws/.worktrees/demo/.harness-git/config",
	}
}

func TestValidateSpec(t *testing.T) {
	t.Parallel()

	if err := validSpec().Validate(); err != nil {
		t.Fatal(err)
	}
	escalated := validSpec()
	escalated.Effort = "xhigh"
	if err := escalated.Validate(); err != nil {
		t.Fatalf("xhigh must validate for escalations: %v", err)
	}
}

func TestRejectBadSpecs(t *testing.T) {
	t.Parallel()

	mk := func(mut func(*ThreadSpec)) ThreadSpec {
		spec := validSpec()
		mut(&spec)
		return spec
	}
	cases := []struct {
		name string
		spec ThreadSpec
	}{
		{name: "empty role", spec: mk(func(s *ThreadSpec) { s.Role = "" })},
		{name: "bad role chars", spec: mk(func(s *ThreadSpec) { s.Role = "evil role" })},
		{name: "empty objective", spec: mk(func(s *ThreadSpec) { s.Objective = "" })},
		{name: "objective too long", spec: mk(func(s *ThreadSpec) { s.Objective = strings.Repeat("x", 2001) })},
		{name: "empty model", spec: mk(func(s *ThreadSpec) { s.Model = "" })},
		{name: "model with space", spec: mk(func(s *ThreadSpec) { s.Model = "my model" })},
		{name: "bad effort", spec: mk(func(s *ThreadSpec) { s.Effort = "turbo" })},
		{name: "confidential", spec: mk(func(s *ThreadSpec) { s.Classification = "confidential" })},
		{name: "restricted", spec: mk(func(s *ThreadSpec) { s.Classification = "restricted" })},
		{name: "workdir outside root", spec: mk(func(s *ThreadSpec) { s.Workdir = "/other/dir" })},
		{name: "writable outside root", spec: mk(func(s *ThreadSpec) { s.WritablePaths = []string{"/etc/passwd"} })},
		{name: "bad skill", spec: mk(func(s *ThreadSpec) { s.Skills = []string{"evil skill"} })},
		{name: "zero budget", spec: mk(func(s *ThreadSpec) { s.BudgetUSD = 0 })},
		{name: "bad timeout", spec: mk(func(s *ThreadSpec) { s.Timeout = 30 * time.Second })},
		{name: "relative git config", spec: mk(func(s *ThreadSpec) { s.GitConfigGlobal = "relative/config" })},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.spec.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestPromptRendersEnvelope(t *testing.T) {
	t.Parallel()

	got := Prompt(validSpec())
	for _, want := range []string{
		"Role: implementer",
		"Objective: Add the missing validation",
		"Spec: owner/repo#123",
		"Data classification: internal",
		"/ws/.worktrees/demo",
		"harness/gates",
		"restricted",
		"go test ./... passes",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}
}
