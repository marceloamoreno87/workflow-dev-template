package telemetry

import (
	"strings"
	"testing"
	"time"
)

type EvalCase struct {
	TaskKind string
	Model    string
	Effort   string
}

func runEval(cases []EvalCase) []string {
	var failures []string
	for _, c := range cases {
		route, err := Select(c.TaskKind, "internal")
		if err != nil || route.Model != c.Model || route.Effort != c.Effort {
			failures = append(failures, c.TaskKind)
		}
	}
	return failures
}

func TestRoutingBaselineEval(t *testing.T) {
	t.Parallel()

	cases := []EvalCase{
		{TaskKind: "triage", Model: "gpt-5.6-luna", Effort: "low"},
		{TaskKind: "summarize", Model: "gpt-5.6-luna", Effort: "low"},
		{TaskKind: "index", Model: "gpt-5.6-luna", Effort: "low"},
		{TaskKind: "mechanical", Model: "gpt-5.6-luna", Effort: "low"},
		{TaskKind: "explore", Model: "gpt-5.6-terra", Effort: "low"},
		{TaskKind: "spec", Model: "gpt-5.6-terra", Effort: "medium"},
		{TaskKind: "implement", Model: "gpt-5.6-terra", Effort: "medium"},
		{TaskKind: "fix", Model: "gpt-5.6-terra", Effort: "medium"},
		{TaskKind: "review", Model: "gpt-5.6-terra", Effort: "high"},
		{TaskKind: "architecture", Model: "gpt-5.6-sol", Effort: "high"},
		{TaskKind: "cross-cutting", Model: "gpt-5.6-sol", Effort: "medium"},
		{TaskKind: "security-review", Model: "gpt-5.6-sol", Effort: "high"},
		{TaskKind: "critical-review", Model: "gpt-5.6-sol", Effort: "high"},
		{TaskKind: "escalate", Model: "gpt-5.6-sol", Effort: "xhigh"},
	}
	if failures := runEval(cases); len(failures) > 0 {
		t.Fatalf("baseline drift: %v", failures)
	}
}

func TestObservabilityFlow(t *testing.T) {
	t.Parallel()

	route, err := Select("implement", "internal")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	span, err := NewSpan("implement", now)
	if err != nil {
		t.Fatal(err)
	}
	span.WorkItem = "owner/repo#1"
	span.Attrs = map[string]string{"model": route.Model, "effort": route.Effort, "api_key": "s3cr3t"}
	span.Finish(now.Add(2 * time.Minute))
	rec := NewRecorder()
	if err := rec.Record(span); err != nil {
		t.Fatal(err)
	}
	attribution, err := Attribute([]UsageRecord{{
		WorkItem: "owner/repo#1", Model: route.Model,
		InputTokens: 1200, OutputTokens: 300, CostUSD: 0.04, Duration: 2 * time.Minute,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if attribution.ByModel[route.Model].CostUSD != 0.04 {
		t.Fatalf("cost lost: %#v", attribution)
	}
	var b strings.Builder
	if err := rec.Export(&b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "s3cr3t") {
		t.Fatalf("secret reached export:\n%s", b.String())
	}
}
