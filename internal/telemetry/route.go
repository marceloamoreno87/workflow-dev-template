// internal/telemetry/route.go
package telemetry

import (
	"errors"
	"fmt"
)

var ErrTelemetry = errors.New("invalid telemetry input")

type Route struct {
	Model  string
	Effort string
}

var baseline = map[string]Route{
	"triage":          {Model: "gpt-5.6-luna", Effort: "low"},
	"summarize":       {Model: "gpt-5.6-luna", Effort: "low"},
	"index":           {Model: "gpt-5.6-luna", Effort: "low"},
	"mechanical":      {Model: "gpt-5.6-luna", Effort: "low"},
	"explore":         {Model: "gpt-5.6-terra", Effort: "low"},
	"spec":            {Model: "gpt-5.6-terra", Effort: "medium"},
	"implement":       {Model: "gpt-5.6-terra", Effort: "medium"},
	"fix":             {Model: "gpt-5.6-terra", Effort: "medium"},
	"review":          {Model: "gpt-5.6-terra", Effort: "high"},
	"architecture":    {Model: "gpt-5.6-sol", Effort: "high"},
	"cross-cutting":   {Model: "gpt-5.6-sol", Effort: "medium"},
	"security-review": {Model: "gpt-5.6-sol", Effort: "high"},
	"critical-review": {Model: "gpt-5.6-sol", Effort: "high"},
	"escalate":        {Model: "gpt-5.6-sol", Effort: "xhigh"},
}

func Select(taskKind, classification string) (Route, error) {
	switch classification {
	case "public", "internal":
	default:
		return Route{}, fmt.Errorf("%w: classification %q never routes to a model", ErrTelemetry, classification)
	}
	route, ok := baseline[taskKind]
	if !ok {
		return Route{}, fmt.Errorf("%w: unknown task kind %q", ErrTelemetry, taskKind)
	}
	return route, nil
}
