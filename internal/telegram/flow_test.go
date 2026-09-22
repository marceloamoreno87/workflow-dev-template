package telegram

import (
	"testing"
	"time"
)

func TestAuthenticatedSensitiveFlow(t *testing.T) {
	t.Parallel()

	now := time.Now()
	secret := []byte("flow-secret-at-least-16")
	in, err := ParseUpdate([]byte(updateJSON(1001, 2002, "private", "/approve owner/repo#123 4 ship it", now.Unix())), validConfig(), now)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := ParseCommand(in)
	if err != nil {
		t.Fatal(err)
	}
	if !intent.Sensitive {
		t.Fatal("approve must be sensitive")
	}
	ch, err := NewChallenge(secret, "actor/operator", intent.Type, intent.ExpectedVersion, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyChallenge(secret, ch, "actor/operator", intent.Type, intent.ExpectedVersion, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func TestFlowRejectsEverythingElse(t *testing.T) {
	t.Parallel()

	now := time.Now()
	// Stranger, stale, unknown command, expired challenge: four rejections, no exceptions.
	if _, err := ParseUpdate([]byte(updateJSON(9999, 2002, "private", "/approve owner/repo#123 4", now.Unix())), validConfig(), now); err == nil {
		t.Fatal("stranger accepted")
	}
	if _, err := ParseUpdate([]byte(updateJSON(1001, 2002, "private", "/approve owner/repo#123 4", now.Add(-time.Hour).Unix())), validConfig(), now); err == nil {
		t.Fatal("stale accepted")
	}
	if _, err := ParseCommand(Inbound{UserID: 1001, ChatID: 2002, Text: "/deploy owner/repo#123 4"}); err == nil {
		t.Fatal("deploy accepted over telegram")
	}
	secret := []byte("flow-secret-at-least-16")
	ch, err := NewChallenge(secret, "actor/operator", "approve_pr", 4, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyChallenge(secret, ch, "actor/operator", "approve_pr", 4, now.Add(time.Hour)); err == nil {
		t.Fatal("expired challenge accepted")
	}
}
