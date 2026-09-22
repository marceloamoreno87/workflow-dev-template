// internal/telemetry/span.go
package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const maxSpans = 4096

var secretSubstrings = []string{"token", "secret", "password", "credentials", "private_key"}

type Span struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Name       string
	WorkItem   string
	StartedAt  time.Time
	FinishedAt time.Time
	Attrs      map[string]string
}

func hexID(nbytes int) (string, error) {
	raw := make([]byte, nbytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func NewSpan(name string, startedAt time.Time) (Span, error) {
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 256 {
		return Span{}, fmt.Errorf("%w: span name", ErrTelemetry)
	}
	trace, err := hexID(16)
	if err != nil {
		return Span{}, err
	}
	id, err := hexID(8)
	if err != nil {
		return Span{}, err
	}
	return Span{TraceID: trace, SpanID: id, Name: name, StartedAt: startedAt}, nil
}

// Finish closes the span window; the caller owns the clock.
func (s *Span) Finish(finishedAt time.Time) {
	s.FinishedAt = finishedAt
}

func redact(attrs map[string]string) map[string]string {
	if attrs == nil {
		return nil
	}
	out := make(map[string]string, len(attrs))
	for key, value := range attrs {
		lower := strings.ToLower(key)
		redacted := false
		for _, secret := range secretSubstrings {
			if strings.Contains(lower, secret) {
				redacted = true
				break
			}
		}
		if redacted {
			out[key] = "[redacted]"
			continue
		}
		out[key] = value
	}
	return out
}

func (s Span) validate() error {
	if len(s.TraceID) != 32 || len(s.SpanID) != 16 {
		return fmt.Errorf("%w: span identity", ErrTelemetry)
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("%w: span name", ErrTelemetry)
	}
	if s.FinishedAt.Before(s.StartedAt) {
		return fmt.Errorf("%w: span window inverted", ErrTelemetry)
	}
	if len(s.Attrs) > 32 {
		return fmt.Errorf("%w: too many attributes", ErrTelemetry)
	}
	return nil
}

type Recorder struct {
	spans []Span
}

func NewRecorder() *Recorder {
	return &Recorder{}
}

func (r *Recorder) Record(span Span) error {
	if err := span.validate(); err != nil {
		return err
	}
	if len(r.spans) >= maxSpans {
		return fmt.Errorf("%w: recorder full", ErrTelemetry)
	}
	span.Attrs = redact(span.Attrs)
	r.spans = append(r.spans, span)
	return nil
}

func (r *Recorder) Export(w io.Writer) error {
	for _, span := range r.spans {
		raw, err := json.Marshal(map[string]any{
			"traceId":  span.TraceID,
			"spanId":   span.SpanID,
			"parentId": span.ParentID,
			"name":     span.Name,
			"workItem": span.WorkItem,
			"started":  span.StartedAt.UTC().Format(time.RFC3339Nano),
			"finished": span.FinishedAt.UTC().Format(time.RFC3339Nano),
			"attrs":    span.Attrs,
		})
		if err != nil {
			return err
		}
		if _, err := w.Write(append(raw, '\n')); err != nil {
			return err
		}
	}
	return nil
}
