package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCommandIntake(t *testing.T) {
	_, server := testServer(t)
	host := strings.TrimPrefix(server.URL, "http://")

	post := func(body string) (int, map[string]any) {
		t.Helper()
		req, _ := http.NewRequest("POST", server.URL+"/api/commands", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://"+host)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var doc map[string]any
		_ = json.Unmarshal(raw, &doc)
		return res.StatusCode, doc
	}

	code, doc := post(`{"aggregateId":"owner/repo#123","expectedVersion":4,"type":"begin_triage","reason":""}`)
	if code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %v", code, doc)
	}
	if doc["actorId"] != "actor/operator" || doc["aggregateId"] != "owner/repo#123" {
		t.Fatalf("unexpected echo: %v", doc)
	}
	for _, body := range []string{
		`{}`,
		`{"aggregateId":"","expectedVersion":4,"type":"begin_triage"}`,
		`{"aggregateId":"owner/repo#123","expectedVersion":4,"type":"teleport"}`,
		`{"aggregateId":"owner/repo#123","expectedVersion":4,"type":"cancel"}`,
		`not json`,
	} {
		if code, _ := post(body); code != http.StatusBadRequest && code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 4xx for %q, got %d", body, code)
		}
	}
}
