package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/telegram"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func testTelegramConfig(t *testing.T, baseURL string) *TelegramConfig {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bot"), []byte("bot-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret"), []byte("challenge-secret-16"), 0o600); err != nil {
		t.Fatal(err)
	}
	botToken, err := os.ReadFile(filepath.Join(dir, "bot"))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := os.ReadFile(filepath.Join(dir, "secret"))
	if err != nil {
		t.Fatal(err)
	}
	return &TelegramConfig{
		BaseURL:         baseURL,
		AllowedUsers:    []int64{1001},
		AllowedChats:    []int64{2002},
		BotToken:        string(botToken),
		ChallengeSecret: secret,
	}
}

// seedReviewing drives an item to reviewing through direct journal applies,
// simulating surfaces plus role execution.
func seedReviewing(t *testing.T, d *Daemon, id workflow.WorkItemID) {
	t.Helper()

	now := time.Now()
	steps := []workflow.Command{
		{ID: "seed-1", AggregateID: id, ExpectedVersion: 0, ActorID: "actor/automation", Type: workflow.CommandSubmitWork},
		{ID: "seed-2", AggregateID: id, ExpectedVersion: 1, ActorID: "actor/automation", Type: workflow.CommandBeginTriage},
		{ID: "seed-3", AggregateID: id, ExpectedVersion: 2, ActorID: "actor/operator", Type: workflow.CommandAuthorizeWork},
		{ID: "seed-4", AggregateID: id, ExpectedVersion: 3, ActorID: "actor/automation", Type: workflow.CommandBeginImplementation},
		{ID: "seed-5", AggregateID: id, ExpectedVersion: 4, ActorID: "actor/automation", Type: workflow.CommandSubmitReview},
	}
	for _, cmd := range steps {
		if _, err := d.Journal().Apply(now, cmd); err != nil {
			t.Fatal(err)
		}
	}
}

func telegramDouble(t *testing.T, updates string, sent *[]string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			_, _ = w.Write([]byte(`{"ok":true,"result":[` + updates + `]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			var doc struct {
				Text string `json:"text"`
			}
			_ = json.NewDecoder(r.Body).Decode(&doc)
			*sent = append(*sent, doc.Text)
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func telegramDaemon(t *testing.T, server *httptest.Server) *Daemon {
	t.Helper()

	root := t.TempDir()
	cfg := testConfig(root)
	cfg.Telegram = testTelegramConfig(t, server.URL)
	d, err := Open(cfg, &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestTelegramPlainCommandApplies(t *testing.T) {
	now := time.Now()
	update := `{"update_id":41,"message":{"message_id":7,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":"/changes owner/repo#1 5 needs tests","date":` + itoa(now.Unix()) + `}}`
	var sent []string
	server := telegramDouble(t, update, &sent)
	defer server.Close()

	d := telegramDaemon(t, server)
	seedReviewing(t, d, "owner/repo#1")
	if err := d.pollTelegram(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := mustLoad(t, d, "owner/repo#1"); got.State != workflow.StateChangesRequested {
		t.Fatalf("plain command not applied: %#v", got)
	}
	if len(sent) != 0 {
		t.Fatalf("no message expected, got %v", sent)
	}
	if raw, err := os.ReadFile(filepath.Join(d.cfg.WorkspaceRoot, ".harness", "telegram-offset")); err != nil || strings.TrimSpace(string(raw)) != "42" {
		t.Fatalf("offset not persisted: %q %v", raw, err)
	}
}

func TestTelegramSensitiveNeedsChallenge(t *testing.T) {
	now := time.Now()
	update := `{"update_id":42,"message":{"message_id":7,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":"/approve owner/repo#1 5","date":` + itoa(now.Unix()) + `}}`
	var sent []string
	server := telegramDouble(t, update, &sent)
	defer server.Close()

	d := telegramDaemon(t, server)
	seedReviewing(t, d, "owner/repo#1")
	if err := d.pollTelegram(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := mustLoad(t, d, "owner/repo#1"); got.State != workflow.StateReviewing {
		t.Fatalf("sensitive command applied without challenge: %#v", got)
	}
	if len(sent) != 1 || !strings.Contains(sent[0], "challenge") {
		t.Fatalf("challenge not offered: %v", sent)
	}
}

func TestTelegramChallengeAccepts(t *testing.T) {
	now := time.Now()
	secret := []byte("challenge-secret-16")
	issued, err := telegram.NewChallenge(secret, "actor/operator", "approve_pr", 5, now)
	if err != nil {
		t.Fatal(err)
	}
	text := "/approve owner/repo#1 5 challenge " + issued.ID + " " + itoa(issued.ExpiresAt.Unix()) + " ship it"
	update := `{"update_id":43,"message":{"message_id":7,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":` + strconv.Quote(text) + `,"date":` + itoa(now.Unix()) + `}}`
	var sent []string
	server := telegramDouble(t, update, &sent)
	defer server.Close()

	d := telegramDaemon(t, server)
	seedReviewing(t, d, "owner/repo#1")
	if err := d.pollTelegram(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	got := mustLoad(t, d, "owner/repo#1")
	if got.State != workflow.StateReadyToDeploy || got.Version != 6 {
		t.Fatalf("challenged approve not applied: %#v", got)
	}
	if len(sent) != 0 {
		t.Fatalf("no new challenge expected, got %v", sent)
	}
}
