// internal/daemon/telegram.go
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/dashboard"
	"github.com/marceloamoreno87/workflow-dev-template/internal/telegram"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

const offsetFileName = "telegram-offset"

func (d *Daemon) offsetPath() string {
	return filepath.Join(harnessDir(d.cfg.WorkspaceRoot), offsetFileName)
}

func (d *Daemon) loadOffset() (int64, error) {
	raw, err := os.ReadFile(d.offsetPath())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	offset, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("%w: corrupt offset", ErrDaemon)
	}
	return offset, nil
}

func (d *Daemon) saveOffset(offset int64) error {
	return os.WriteFile(d.offsetPath(), []byte(strconv.FormatInt(offset, 10)), 0o644)
}

// pollTelegram ingests one batch of Bot updates: plain intents apply through the
// operator path, sensitive intents need a valid embedded challenge (otherwise a
// fresh challenge is offered via sendMessage and nothing is recorded). The offset
// advances past every seen update, including skipped ones.
func (d *Daemon) pollTelegram(ctx context.Context, now time.Time) error {
	cfg := d.cfg.Telegram
	if cfg == nil {
		return nil
	}
	client, err := telegram.NewClient(cfg.BaseURL, cfg.BotToken)
	if err != nil {
		return err
	}
	offset, err := d.loadOffset()
	if err != nil {
		return err
	}
	updates, err := client.GetUpdates(ctx, offset, 0)
	if err != nil {
		return err
	}
	botCfg := telegram.Config{AllowedUsers: cfg.AllowedUsers, AllowedChats: cfg.AllowedChats}
	for _, update := range updates {
		if update.UpdateID+1 > offset {
			offset = update.UpdateID + 1
		}
		d.handleTelegramUpdate(ctx, now, client, botCfg, cfg.ChallengeSecret, update.Message)
	}
	return d.saveOffset(offset)
}

func (d *Daemon) handleTelegramUpdate(ctx context.Context, now time.Time, client telegram.Client, botCfg telegram.Config, secret []byte, raw json.RawMessage) {
	full, err := json.Marshal(map[string]json.RawMessage{"message": raw})
	if err != nil {
		return
	}
	in, err := telegram.ParseUpdate(full, botCfg, now)
	if err != nil {
		return
	}
	intent, err := telegram.ParseCommand(in)
	if err != nil {
		return
	}
	if !intent.Sensitive {
		_, _ = d.ApplyOperatorCommand(dashboard.CommandRequest{
			AggregateID:     intent.AggregateID,
			ExpectedVersion: intent.ExpectedVersion,
			Type:            workflow.CommandType(intent.Type),
			Reason:          intent.Reason,
		})
		return
	}
	challengeID, expires, rest, ok := splitChallenge(intent.Reason)
	if !ok {
		issued, err := telegram.NewChallenge(secret, "actor/operator", intent.Type, intent.ExpectedVersion, now)
		if err != nil {
			return
		}
		_ = client.SendMessage(ctx, in.ChatID, fmt.Sprintf(
			"Sensitive command needs a fresh challenge. Reply: %s %s %d challenge %s %d",
			commandName(intent.Type), intent.AggregateID, intent.ExpectedVersion,
			issued.ID, issued.ExpiresAt.Unix(),
		))
		return
	}
	if err := telegram.VerifyChallenge(secret, telegram.Challenge{ID: challengeID, ExpiresAt: time.Unix(expires, 0)}, "actor/operator", intent.Type, intent.ExpectedVersion, now); err != nil {
		return
	}
	_, _ = d.ApplyOperatorCommand(dashboard.CommandRequest{
		AggregateID:     intent.AggregateID,
		ExpectedVersion: intent.ExpectedVersion,
		Type:            workflow.CommandType(intent.Type),
		Reason:          rest,
	})
}

func splitChallenge(reason string) (id string, expires int64, rest string, ok bool) {
	fields := strings.Fields(reason)
	if len(fields) < 3 || fields[0] != "challenge" {
		return "", 0, "", false
	}
	exp, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || exp <= 0 {
		return "", 0, "", false
	}
	if fields[1] == "" {
		return "", 0, "", false
	}
	return fields[1], exp, strings.TrimSpace(strings.Join(fields[3:], " ")), true
}

func commandName(cmdType string) string {
	switch workflow.CommandType(cmdType) {
	case workflow.CommandApprovePR:
		return "/approve"
	case workflow.CommandAcceptFeature:
		return "/accept"
	case workflow.CommandRequestChanges:
		return "/changes"
	case workflow.CommandRetryDeployment:
		return "/retry"
	case workflow.CommandCancel:
		return "/cancel"
	default:
		return "/unknown"
	}
}
