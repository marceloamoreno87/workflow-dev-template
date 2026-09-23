package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postAuthed(t *testing.T, base, path, body string) (int, map[string]any) {
	t.Helper()

	host := strings.TrimPrefix(base, "http://")
	req, _ := http.NewRequest("POST", base+path, strings.NewReader(body))
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

func applyServer(t *testing.T, apply ApplyFunc) *httptest.Server {
	t.Helper()

	srv, err := NewServer(Config{BindAddr: "127.0.0.1:0", Token: "operator-token-at-least-16", Apply: apply})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(srv.Handler())
	t.Cleanup(server.Close)
	return server
}

func TestApplyCommits(t *testing.T) {
	server := applyServer(t, func(req CommandRequest) (Applied, error) {
		if req.Type != "begin_triage" || req.AggregateID != "owner/repo#123" {
			t.Errorf("unexpected request: %#v", req)
		}
		return Applied{Version: 1}, nil
	})

	code, doc := postAuthed(t, server.URL, "/api/commands", `{"aggregateId":"owner/repo#123","expectedVersion":0,"type":"begin_triage"}`)
	if code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %v", code, doc)
	}
	if doc["version"] != float64(1) || doc["actorId"] != "actor/operator" || doc["status"] != "applied" {
		t.Fatalf("unexpected echo: %v", doc)
	}
}

func TestApplyMapsErrors(t *testing.T) {
	conflict := applyServer(t, func(req CommandRequest) (Applied, error) {
		return Applied{}, ErrApplyConflict
	})
	if code, _ := postAuthed(t, conflict.URL, "/api/commands", `{"aggregateId":"owner/repo#123","expectedVersion":0,"type":"begin_triage"}`); code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", code)
	}

	rejected := applyServer(t, func(req CommandRequest) (Applied, error) {
		return Applied{}, ErrApplyRejected
	})
	if code, _ := postAuthed(t, rejected.URL, "/api/commands", `{"aggregateId":"owner/repo#123","expectedVersion":0,"type":"begin_triage"}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", code)
	}

	bare, err := NewServer(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	bareServer := httptest.NewServer(bare.Handler())
	defer bareServer.Close()
	if code, _ := postAuthed(t, bareServer.URL, "/api/commands", `{"aggregateId":"owner/repo#123","expectedVersion":0,"type":"begin_triage"}`); code != http.StatusNotImplemented {
		t.Fatalf("expected 501 without applier, got %d", code)
	}
}
