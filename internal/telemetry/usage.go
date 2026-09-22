// internal/telemetry/usage.go
package telemetry

import (
	"fmt"
	"time"
)

type UsageRecord struct {
	WorkItem     string
	Model        string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	Duration     time.Duration
}

type UsageTotal struct {
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	Duration     time.Duration
}

type Attribution struct {
	ByWorkItem map[string]UsageTotal
	ByModel    map[string]UsageTotal
	Total      UsageTotal
}

func (r UsageRecord) Validate() error {
	if r.WorkItem == "" {
		return fmt.Errorf("%w: work item required", ErrTelemetry)
	}
	if r.Model == "" {
		return fmt.Errorf("%w: model required", ErrTelemetry)
	}
	if r.InputTokens < 0 || r.OutputTokens < 0 {
		return fmt.Errorf("%w: negative tokens", ErrTelemetry)
	}
	if r.CostUSD < 0 {
		return fmt.Errorf("%w: negative cost", ErrTelemetry)
	}
	if r.Duration < 0 {
		return fmt.Errorf("%w: negative duration", ErrTelemetry)
	}
	return nil
}

func (t *UsageTotal) add(r UsageRecord) {
	t.InputTokens += r.InputTokens
	t.OutputTokens += r.OutputTokens
	t.CostUSD += r.CostUSD
	t.Duration += r.Duration
}

func Attribute(records []UsageRecord) (Attribution, error) {
	if len(records) == 0 {
		return Attribution{}, fmt.Errorf("%w: no usage records", ErrTelemetry)
	}
	out := Attribution{ByWorkItem: map[string]UsageTotal{}, ByModel: map[string]UsageTotal{}}
	for _, r := range records {
		if err := r.Validate(); err != nil {
			return Attribution{}, err
		}
		work := out.ByWorkItem[r.WorkItem]
		work.add(r)
		out.ByWorkItem[r.WorkItem] = work
		model := out.ByModel[r.Model]
		model.add(r)
		out.ByModel[r.Model] = model
		out.Total.add(r)
	}
	return out, nil
}
