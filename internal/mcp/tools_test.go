package mcp

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func itoa(v int) string {
	return strconv.Itoa(v)
}

func authedServer(t *testing.T, backend *fakeBackend) *Server {
	t.Helper()

	srv := NewServer("execution-token-at-least-16", "owner/repo#1", backend)
	srv.Handle([]byte(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`))
	return srv
}

func call(t *testing.T, srv *Server, id int, name, args string) map[string]any {
	t.Helper()

	raw := srv.Handle([]byte(`{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`))
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestReadWork(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "read_work", `{"workItem":"owner/repo#1"}`)
	result, _ := doc["result"].(map[string]any)
	if result["id"] != "owner/repo#1" || result["state"] != "implementing" {
		t.Fatalf("unexpected work view: %v", doc)
	}
	doc = call(t, srv, 2, "read_work", `{"workItem":"owner/repo#2"}`)
	if _, ok := doc["error"]; !ok {
		t.Fatalf("cross-item read allowed: %v", doc)
	}
	if backend.reads != 1 {
		t.Fatalf("scope check must precede backend: reads=%d", backend.reads)
	}
}

func TestGetSpec(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "get_spec", `{"workItem":"owner/repo#1"}`)
	result, _ := doc["result"].(map[string]any)
	if result["ref"] != "owner/repo#1" || !strings.Contains(result["body"].(string), "health") {
		t.Fatalf("unexpected spec: %v", doc)
	}
}

func TestSearchKnowledge(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "search_knowledge", `{"query":"freeze","limit":5}`)
	result, _ := doc["result"].(map[string]any)
	hits, _ := result["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("unexpected hits: %v", doc)
	}
	if backend.lastLimit != 5 {
		t.Fatalf("limit not passed through: %d", backend.lastLimit)
	}
	doc = call(t, srv, 2, "search_knowledge", `{"query":""}`)
	if _, ok := doc["error"]; !ok {
		t.Fatalf("empty query allowed: %v", doc)
	}
}
