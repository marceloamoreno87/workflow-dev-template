package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranscriptIsCapped(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	bin := withStubCodex(t, "ok")

	var events strings.Builder
	for i := 0; i < transcriptEventCap+500; i++ {
		events.WriteString(`{"type":"item.completed","n":"` + strings.Repeat("0", 10) + "\"}\n")
	}
	if err := os.WriteFile(filepath.Join(bin, "events.jsonl"), []byte(events.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if !res.TranscriptTruncated || res.TranscriptEvents != transcriptEventCap {
		t.Fatalf("transcript not capped: %#v", res)
	}
}

func TestClassifiedSpecNeverSpawns(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel tests.
	bin := withStubCodex(t, "ok")

	spec := validSpec()
	spec.Classification = "restricted"
	if _, err := Run(context.Background(), spec); err == nil {
		t.Fatal("expected classification rejection, got none")
	}
	if raw, err := os.ReadFile(filepath.Join(bin, "argv.log")); err == nil {
		t.Fatalf("classified spec reached the binary:\n%s", raw)
	}
}

func TestFixturePathIsGated(t *testing.T) {
	if os.Getenv("HARNESS_CODEX_FIXTURE") == "" {
		t.Skip("set HARNESS_CODEX_FIXTURE=1 with local auth to run the fixture thread")
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		t.Skip("CODEX_HOME is not set")
	}
	if _, err := os.Stat(filepath.Join(home, "auth.json")); err != nil {
		t.Skip("no local codex auth present")
	}
	if os.Getenv("HARNESS_CODEX_LIVE") == "" {
		t.Log("auth present; set HARNESS_CODEX_LIVE=1 to spend model budget on the fixture thread")
		t.Skip("live-model gate closed")
	}
}
