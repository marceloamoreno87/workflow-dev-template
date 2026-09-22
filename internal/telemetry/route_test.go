package telemetry

import (
	"testing"
)

func TestSelectBaseline(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		kind   string
		model  string
		effort string
	}{
		{kind: "triage", model: "gpt-5.6-luna", effort: "low"},
		{kind: "summarize", model: "gpt-5.6-luna", effort: "low"},
		{kind: "index", model: "gpt-5.6-luna", effort: "low"},
		{kind: "mechanical", model: "gpt-5.6-luna", effort: "low"},
		{kind: "explore", model: "gpt-5.6-terra", effort: "low"},
		{kind: "spec", model: "gpt-5.6-terra", effort: "medium"},
		{kind: "implement", model: "gpt-5.6-terra", effort: "medium"},
		{kind: "fix", model: "gpt-5.6-terra", effort: "medium"},
		{kind: "review", model: "gpt-5.6-terra", effort: "high"},
		{kind: "architecture", model: "gpt-5.6-sol", effort: "high"},
		{kind: "cross-cutting", model: "gpt-5.6-sol", effort: "medium"},
		{kind: "security-review", model: "gpt-5.6-sol", effort: "high"},
		{kind: "critical-review", model: "gpt-5.6-sol", effort: "high"},
		{kind: "escalate", model: "gpt-5.6-sol", effort: "xhigh"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			route, err := Select(tc.kind, "internal")
			if err != nil {
				t.Fatal(err)
			}
			if route.Model != tc.model || route.Effort != tc.effort {
				t.Fatalf("got %#v", route)
			}
		})
	}
}

func TestSelectRejects(t *testing.T) {
	t.Parallel()

	if _, err := Select("teleport", "internal"); err == nil {
		t.Fatal("expected unknown-kind rejection, got none")
	}
	for _, classification := range []string{"confidential", "restricted", "bogus"} {
		if _, err := Select("triage", classification); err == nil {
			t.Fatalf("expected classification rejection for %q, got none", classification)
		}
	}
	if _, err := Select("triage", "public"); err != nil {
		t.Fatal(err)
	}
}
