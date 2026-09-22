package telemetry

import (
	"strings"
	"testing"
	"time"
)

func TestRecordAndExport(t *testing.T) {
	t.Parallel()

	rec := NewRecorder()
	span, err := NewSpan("implement", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	span.WorkItem = "owner/repo#1"
	span.Finish(time.Now().Add(time.Minute))
	span.Attrs = map[string]string{"model": "gpt-5.6-terra", "github_token": "s3cr3t"}
	if err := rec.Record(span); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := rec.Export(&b); err != nil {
		t.Fatal(err)
	}
	exported := b.String()
	if strings.Contains(exported, "s3cr3t") {
		t.Fatalf("secret reached export:\n%s", exported)
	}
	if !strings.Contains(exported, "[redacted]") || !strings.Contains(exported, span.SpanID) {
		t.Fatalf("redaction or identity missing:\n%s", exported)
	}
}

func TestSpanIDsAreShaped(t *testing.T) {
	t.Parallel()

	span, err := NewSpan("review", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(span.TraceID) != 32 || len(span.SpanID) != 16 {
		t.Fatalf("bad id shapes: %#v", span)
	}
	for _, c := range span.TraceID + span.SpanID {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			t.Fatalf("non-hex id: %#v", span)
		}
	}
}

func TestRejectBadSpans(t *testing.T) {
	t.Parallel()

	now := time.Now()
	if _, err := NewSpan("", now); err == nil {
		t.Fatal("expected empty-name rejection, got none")
	}
	span, err := NewSpan("x", now)
	if err != nil {
		t.Fatal(err)
	}
	span.Finish(now.Add(-time.Second))
	if err := NewRecorder().Record(span); err == nil {
		t.Fatal("expected inverted-window rejection, got none")
	}
}
