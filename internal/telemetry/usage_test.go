package telemetry

import (
	"testing"
	"time"
)

func TestAttributeCost(t *testing.T) {
	t.Parallel()

	records := []UsageRecord{
		{WorkItem: "owner/repo#1", Model: "gpt-5.6-terra", InputTokens: 1000, OutputTokens: 500, CostUSD: 0.5, Duration: time.Minute},
		{WorkItem: "owner/repo#1", Model: "gpt-5.6-luna", InputTokens: 2000, OutputTokens: 100, CostUSD: 0.25, Duration: time.Minute},
		{WorkItem: "owner/repo#2", Model: "gpt-5.6-terra", InputTokens: 500, OutputTokens: 500, CostUSD: 0.125, Duration: time.Minute},
	}
	attribution, err := Attribute(records)
	if err != nil {
		t.Fatal(err)
	}
	if attribution.ByWorkItem["owner/repo#1"].CostUSD != 0.75 {
		t.Fatalf("work item cost wrong: %#v", attribution.ByWorkItem["owner/repo#1"])
	}
	if attribution.ByModel["gpt-5.6-terra"].OutputTokens != 1000 {
		t.Fatalf("model tokens wrong: %#v", attribution.ByModel["gpt-5.6-terra"])
	}
	if attribution.Total.CostUSD != 0.875 || attribution.Total.InputTokens != 3500 {
		t.Fatalf("totals wrong: %#v", attribution.Total)
	}
}

func TestRejectBadUsage(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		record UsageRecord
	}{
		{name: "empty work item", record: UsageRecord{Model: "m", CostUSD: 0.01}},
		{name: "empty model", record: UsageRecord{WorkItem: "w", CostUSD: 0.01}},
		{name: "negative tokens", record: UsageRecord{WorkItem: "w", Model: "m", InputTokens: -1}},
		{name: "negative cost", record: UsageRecord{WorkItem: "w", Model: "m", CostUSD: -0.01}},
		{name: "negative duration", record: UsageRecord{WorkItem: "w", Model: "m", Duration: -time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Attribute([]UsageRecord{tc.record}); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
	if _, err := Attribute(nil); err == nil {
		t.Fatal("expected empty rejection, got none")
	}
}
