package telegram

import (
	"strings"
	"testing"
	"time"
)

func TestChallengeRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now()
	secret := []byte("test-secret-at-least-16")
	ch, err := NewChallenge(secret, "actor/operator", "approve_pr", 4, now)
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID == "" || !ch.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("unexpected challenge: %#v", ch)
	}
	if err := VerifyChallenge(secret, ch, "actor/operator", "approve_pr", 4, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func TestChallengeRejectsMismatch(t *testing.T) {
	t.Parallel()

	now := time.Now()
	secret := []byte("test-secret-at-least-16")
	ch, err := NewChallenge(secret, "actor/operator", "approve_pr", 4, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		actor   string
		gate    string
		version uint64
		at      time.Time
	}{
		{name: "wrong actor", actor: "actor/other", gate: "approve_pr", version: 4, at: now},
		{name: "wrong gate", actor: "actor/operator", gate: "cancel", version: 4, at: now},
		{name: "wrong version", actor: "actor/operator", gate: "approve_pr", version: 5, at: now},
		{name: "expired", actor: "actor/operator", gate: "approve_pr", version: 4, at: now.Add(6 * time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := VerifyChallenge(secret, ch, tc.actor, tc.gate, tc.version, tc.at); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
	if err := VerifyChallenge([]byte("other-secret"), ch, "actor/operator", "approve_pr", 4, now); err == nil {
		t.Fatal("expected secret mismatch rejection, got none")
	}
	if _, err := NewChallenge([]byte("short"), "actor/operator", "approve_pr", 4, now); err == nil {
		t.Fatal("expected weak secret rejection, got none")
	}
	if strings.Contains(ch.ID, "test-secret") {
		t.Fatal("challenge leaks the secret")
	}
}
