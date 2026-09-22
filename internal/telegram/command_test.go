package telegram

import (
	"testing"
)

func TestParseCommand(t *testing.T) {
	t.Parallel()

	in := Inbound{UserID: 1001, ChatID: 2002, Text: "/approve owner/repo#123 4 looks good"}
	intent, err := ParseCommand(in)
	if err != nil {
		t.Fatal(err)
	}
	if intent.Type != "approve_pr" || intent.AggregateID != "owner/repo#123" || intent.ExpectedVersion != 4 || intent.Reason != "looks good" {
		t.Fatalf("unexpected intent: %#v", intent)
	}
	if !intent.Sensitive {
		t.Fatal("approve must be sensitive")
	}
	plain, err := ParseCommand(Inbound{UserID: 1001, ChatID: 2002, Text: "/changes owner/repo#123 4 needs tests"})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Type != "request_changes" || plain.Sensitive {
		t.Fatalf("unexpected plain intent: %#v", plain)
	}
}

func TestRejectBadCommands(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"approve owner/repo#123 4",
		"/deploy owner/repo#123 4",
		"/approve",
		"/approve owner/repo#123",
		"/approve owner/repo#123 four",
		"/approve  4",
		"/approve owner/repo#123 0",
	} {
		if _, err := ParseCommand(Inbound{UserID: 1001, ChatID: 2002, Text: text}); err == nil {
			t.Fatalf("expected rejection for %q, got none", text)
		}
	}
}
