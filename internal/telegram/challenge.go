// internal/telegram/challenge.go
package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"
)

const challengeTTL = 5 * time.Minute

type Challenge struct {
	ID        string
	ExpiresAt time.Time
}

func mac(secret []byte, actor, gate string, version uint64, expires time.Time) []byte {
	h := hmac.New(sha256.New, secret)
	fmt.Fprintf(h, "%s\x00%s\x00%d\x00%d", actor, gate, version, expires.Unix())
	return h.Sum(nil)
}

func NewChallenge(secret []byte, actor, gate string, version uint64, now time.Time) (Challenge, error) {
	if len(secret) < 16 {
		return Challenge{}, fmt.Errorf("%w: challenge secret too short", ErrTelegram)
	}
	if actor == "" || gate == "" || version == 0 {
		return Challenge{}, fmt.Errorf("%w: challenge needs actor, gate, and version", ErrTelegram)
	}
	expires := now.UTC().Add(challengeTTL)
	return Challenge{ID: hex.EncodeToString(mac(secret, actor, gate, version, expires)), ExpiresAt: expires}, nil
}

func VerifyChallenge(secret []byte, ch Challenge, actor, gate string, version uint64, now time.Time) error {
	if len(secret) < 16 {
		return fmt.Errorf("%w: challenge secret too short", ErrTelegram)
	}
	if !now.Before(ch.ExpiresAt) {
		return fmt.Errorf("%w: challenge expired", ErrTelegram)
	}
	want := mac(secret, actor, gate, version, ch.ExpiresAt)
	got, err := hex.DecodeString(ch.ID)
	if err != nil {
		return fmt.Errorf("%w: malformed challenge", ErrTelegram)
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return fmt.Errorf("%w: challenge mismatch", ErrTelegram)
	}
	return nil
}
