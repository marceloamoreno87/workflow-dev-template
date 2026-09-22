// internal/telegram/update.go
package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrTelegram = errors.New("invalid telegram update")

const maxMessageAge = 5 * time.Minute

type Config struct {
	AllowedUsers []int64
	AllowedChats []int64
}

type Inbound struct {
	UserID int64
	ChatID int64
	Text   string
}

func allowed(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func ParseUpdate(data []byte, cfg Config, now time.Time) (Inbound, error) {
	var doc struct {
		Message *struct {
			From *struct {
				ID int64 `json:"id"`
			} `json:"from"`
			Chat *struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			} `json:"chat"`
			Text string `json:"text"`
			Date int64  `json:"date"`
		} `json:"message"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Inbound{}, fmt.Errorf("%w: malformed update", ErrTelegram)
	}
	if doc.Message == nil || doc.Message.From == nil || doc.Message.Chat == nil {
		return Inbound{}, fmt.Errorf("%w: missing message envelope", ErrTelegram)
	}
	if doc.Message.Chat.Type != "private" {
		return Inbound{}, fmt.Errorf("%w: commands require a private chat", ErrTelegram)
	}
	if !allowed(cfg.AllowedUsers, doc.Message.From.ID) {
		return Inbound{}, fmt.Errorf("%w: unknown user", ErrTelegram)
	}
	if !allowed(cfg.AllowedChats, doc.Message.Chat.ID) {
		return Inbound{}, fmt.Errorf("%w: unknown chat", ErrTelegram)
	}
	text := strings.TrimSpace(doc.Message.Text)
	if n := utf8.RuneCountInString(text); n == 0 || n > 4096 {
		return Inbound{}, fmt.Errorf("%w: text length %d", ErrTelegram, n)
	}
	sent := time.Unix(doc.Message.Date, 0)
	if sent.After(now.Add(time.Minute)) || now.Sub(sent) > maxMessageAge {
		return Inbound{}, fmt.Errorf("%w: stale message", ErrTelegram)
	}
	return Inbound{UserID: doc.Message.From.ID, ChatID: doc.Message.Chat.ID, Text: text}, nil
}
