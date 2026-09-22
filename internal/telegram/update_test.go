package telegram

import (
	"strconv"
	"testing"
	"time"
)

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func validConfig() Config {
	return Config{AllowedUsers: []int64{1001}, AllowedChats: []int64{2002}}
}

func updateJSON(from, chat int64, chatType, text string, date int64) string {
	return `{"update_id":1,"message":{"message_id":7,"from":{"id":` + itoa(from) + `},"chat":{"id":` + itoa(chat) + `,"type":"` + chatType + `"},"text":"` + text + `","date":` + itoa(date) + `}}`
}

func TestParseAllowedUpdate(t *testing.T) {
	t.Parallel()

	now := time.Now()
	text := "/approve owner/repo#123 4"
	in, err := ParseUpdate([]byte(updateJSON(1001, 2002, "private", text, now.Unix())), validConfig(), now)
	if err != nil {
		t.Fatal(err)
	}
	if in.UserID != 1001 || in.ChatID != 2002 || in.Text != text {
		t.Fatalf("unexpected inbound: %#v", in)
	}
}

func TestRejectUntrustedUpdates(t *testing.T) {
	t.Parallel()

	now := time.Now()
	fresh := now.Unix()
	stale := now.Add(-10 * time.Minute).Unix()
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{name: "stranger user", doc: updateJSON(9999, 2002, "private", "/approve owner/repo#123 4", fresh)},
		{name: "stranger chat", doc: updateJSON(1001, 9999, "private", "/approve owner/repo#123 4", fresh)},
		{name: "group chat", doc: updateJSON(1001, 2002, "group", "/approve owner/repo#123 4", fresh)},
		{name: "stale message", doc: updateJSON(1001, 2002, "private", "/approve owner/repo#123 4", stale)},
		{name: "empty text", doc: updateJSON(1001, 2002, "private", "   ", fresh)},
		{name: "not json", doc: `{"message":`},
		{name: "no message", doc: `{"update_id":1}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseUpdate([]byte(tc.doc), validConfig(), now); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
