package mcp

import (
	"strings"
	"testing"
)

func TestRunGate(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "run_gate", `{"workItem":"owner/repo#1","gate":"test"}`)
	result, _ := doc["result"].(map[string]any)
	if result["gate"] != "test" || result["exitCode"] != float64(0) {
		t.Fatalf("unexpected gate result: %v", doc)
	}
	doc = call(t, srv, 2, "run_gate", `{"workItem":"owner/repo#1","gate":"teleport"}`)
	if _, ok := doc["error"]; !ok {
		t.Fatalf("undeclared gate allowed: %v", doc)
	}
	if backend.gates["teleport"] {
		t.Fatal("undeclared gate reached backend")
	}
	// 100 KiB of fake output must arrive capped with its flag.
	doc = call(t, srv, 3, "run_gate", `{"workItem":"owner/repo#1","gate":"smoke"}`)
	result, _ = doc["result"].(map[string]any)
	out, _ := result["output"].(string)
	if len(out) > 64*1024 || result["truncated"] != true {
		t.Fatalf("output not capped: len=%d %v", len(out), doc)
	}
	if !strings.HasPrefix(out, "smoke") {
		t.Fatalf("output mangled: %q", out[:20])
	}
}

func TestRequestGate(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "request_gate", `{"workItem":"owner/repo#1","kind":"merge","reason":"gates green"}`)
	result, _ := doc["result"].(map[string]any)
	if result["commandId"] != "mcp-cmd-1" || result["status"] != "queued" {
		t.Fatalf("unexpected request: %v", doc)
	}
	for _, args := range []string{
		`{"workItem":"owner/repo#1","kind":"deploy-everything","reason":"x"}`,
		`{"workItem":"owner/repo#1","kind":"merge","reason":""}`,
		`{"workItem":"owner/repo#2","kind":"merge","reason":"x"}`,
	} {
		doc := call(t, srv, 2, "request_gate", args)
		if _, ok := doc["error"]; !ok {
			t.Fatalf("invalid request allowed: %s", args)
		}
	}
	if backend.requests != 1 {
		t.Fatalf("validation must precede backend: requests=%d", backend.requests)
	}
}

func TestProposeKnowledge(t *testing.T) {
	backend := &fakeBackend{}
	srv := authedServer(t, backend)

	doc := call(t, srv, 1, "propose_knowledge", `{"conceptId":"deploy-freeze","title":"Deploy Freeze","body":"Freeze Fridays.","sources":["owner/repo#1"],"reason":"operator review"}`)
	if _, ok := doc["result"]; !ok {
		t.Fatalf("valid proposal rejected: %v", doc)
	}
	doc = call(t, srv, 2, "propose_knowledge", `{"conceptId":"deploy-freeze","title":"t","body":"b","sources":[],"reason":"r"}`)
	if _, ok := doc["error"]; !ok {
		t.Fatal("unsourced proposal allowed")
	}
	if backend.proposals != 1 {
		t.Fatalf("validation must precede backend: proposals=%d", backend.proposals)
	}
}
