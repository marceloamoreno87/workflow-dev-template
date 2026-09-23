package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInitializeBindsSession(t *testing.T) {
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", &fakeBackend{})

	if res := srv.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)); res == nil {
		t.Fatal("expected auth error, got notification treatment")
	} else if !strings.Contains(string(res), "not authenticated") {
		t.Fatalf("expected auth gate, got %s", res)
	}
	raw := srv.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"wrong"}}`))
	if !strings.Contains(string(raw), "auth failed") {
		t.Fatalf("expected auth failure, got %s", raw)
	}
	raw = srv.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`))
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["result"]; !ok {
		t.Fatalf("expected result, got %s", raw)
	}
	raw = srv.Handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	doc = nil
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	result, _ := doc["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 6 {
		t.Fatalf("expected 6 tools, got %s", raw)
	}
}

func TestToolsetIsClosed(t *testing.T) {
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", &fakeBackend{})
	srv.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`))
	raw := srv.Handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))

	names := map[string]bool{}
	var doc struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, tool := range doc.Result.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"read_work", "get_spec", "search_knowledge", "run_gate", "request_gate", "propose_knowledge"} {
		if !names[want] {
			t.Fatalf("missing tool %q", want)
		}
	}
	for _, banned := range []string{"exec_shell", "sql", "docker", "github", "coolify", "keyring"} {
		if names[banned] {
			t.Fatalf("forbidden tool %q listed", banned)
		}
	}
}

func TestProtocolErrors(t *testing.T) {
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", &fakeBackend{})

	if raw := srv.Handle([]byte(`not json`)); !strings.Contains(string(raw), "-32700") {
		t.Fatalf("expected parse error, got %s", raw)
	}
	auth := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`
	srv.Handle([]byte(auth))
	if raw := srv.Handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"teleport","params":{}}`)); !strings.Contains(string(raw), "-32601") {
		t.Fatalf("expected method-not-found, got %s", raw)
	}
	huge := `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{"pad":"` + strings.Repeat("x", 1024*1024) + `"}}`
	if raw := srv.Handle([]byte(huge)); !strings.Contains(string(raw), "too large") {
		t.Fatal("expected size rejection")
	}
}
