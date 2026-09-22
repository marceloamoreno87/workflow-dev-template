// internal/telegram/command.go
package telegram

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Intent struct {
	Type            string
	AggregateID     string
	ExpectedVersion uint64
	Reason          string
	Sensitive       bool
}

var commandTypes = map[string]struct {
	kind      string
	sensitive bool
}{
	"approve": {kind: "approve_pr", sensitive: true},
	"accept":  {kind: "accept_feature", sensitive: true},
	"changes": {kind: "request_changes", sensitive: false},
	"retry":   {kind: "retry_deployment", sensitive: true},
	"cancel":  {kind: "cancel", sensitive: true},
}

func ParseCommand(in Inbound) (Intent, error) {
	fields := strings.Fields(in.Text)
	if len(fields) < 3 || !strings.HasPrefix(fields[0], "/") {
		return Intent{}, fmt.Errorf("%w: command shape /<name> <aggregate> <version> [reason]", ErrTelegram)
	}
	mapping, ok := commandTypes[strings.TrimPrefix(fields[0], "/")]
	if !ok {
		return Intent{}, fmt.Errorf("%w: unknown command", ErrTelegram)
	}
	aggregate := fields[1]
	if aggregate == "" || len([]rune(aggregate)) > 256 {
		return Intent{}, fmt.Errorf("%w: aggregate", ErrTelegram)
	}
	version, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil || version == 0 {
		return Intent{}, fmt.Errorf("%w: version must be a positive integer", ErrTelegram)
	}
	reason := strings.TrimSpace(strings.Join(fields[3:], " "))
	if n := utf8.RuneCountInString(reason); n > 2000 {
		return Intent{}, fmt.Errorf("%w: reason too long", ErrTelegram)
	}
	return Intent{Type: mapping.kind, AggregateID: aggregate, ExpectedVersion: version, Reason: reason, Sensitive: mapping.sensitive}, nil
}
