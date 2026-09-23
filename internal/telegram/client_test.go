package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetUpdates(t *testing.T) {
	t.Parallel()

	var gotPath, gotOffset string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if !strings.HasPrefix(gotPath, "/botstub-token/getUpdates") {
			t.Errorf("path leaks or wrong: %s", gotPath)
		}
		gotOffset = r.URL.Query().Get("offset")
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":41,"message":{"message_id":7,"from":{"id":1001},"chat":{"id":2002,"type":"private"},"text":"/changes owner/repo#1 3 tests","date":` + itoa(time.Now().Unix()) + `}}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	updates, err := client.GetUpdates(context.Background(), 40, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 41 {
		t.Fatalf("unexpected updates: %#v", updates)
	}
	if gotOffset != "40" {
		t.Fatalf("offset not sent: %q", gotOffset)
	}
}

func TestSendMessage(t *testing.T) {
	t.Parallel()

	var gotPath, gotText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var doc struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&doc)
		gotText = doc.Text
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendMessage(context.Background(), 2002, "hello"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/botstub-token/sendMessage" || gotText != "hello" {
		t.Fatalf("unexpected send: %s %q", gotPath, gotText)
	}
}

func TestClientRejects(t *testing.T) {
	t.Parallel()

	if _, err := NewClient("", "tok"); err == nil {
		t.Fatal("expected url rejection, got none")
	}
	if _, err := NewClient("https://api.example", ""); err == nil {
		t.Fatal("expected token rejection, got none")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"boom"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetUpdates(context.Background(), 0, 0); err == nil {
		t.Fatal("expected ok:false failure, got none")
	} else if strings.Contains(err.Error(), "s3cr3t") {
		t.Fatalf("secret leaked: %v", err)
	}
}
