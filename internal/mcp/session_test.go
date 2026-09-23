package mcp

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

func TestFullSession(t *testing.T) {
	backend := &fakeBackend{}
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", backend)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"execution-token-at-least-16"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_work","arguments":{"workItem":"owner/repo#1"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"request_gate","arguments":{"workItem":"owner/repo#1","kind":"merge","reason":"gates green"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"propose_knowledge","arguments":{"conceptId":"c","title":"t","body":"b","sources":["s"],"reason":"r"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
	}, "\n") + "\n"

	var output strings.Builder
	if err := srv.Serve(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 responses (notification is silent), got %d:\n%s", len(lines), output.String())
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[4]), &last); err != nil {
		t.Fatal(err)
	}
	if _, ok := last["result"]; !ok {
		t.Fatalf("proposal failed: %s", lines[4])
	}
	if backend.requests != 1 || backend.proposals != 1 {
		t.Fatalf("backend not driven: %+v", backend)
	}
}

func TestWrongTokenLearnsNothing(t *testing.T) {
	backend := &fakeBackend{}
	srv := NewServer("execution-token-at-least-16", "owner/repo#1", backend)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"authToken":"wrong"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_work","arguments":{"workItem":"owner/repo#1"}}}`,
	}, "\n") + "\n"

	var output strings.Builder
	if err := srv.Serve(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(output.String())))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `"result"`) {
			t.Fatalf("unauthenticated session returned data: %s", line)
		}
	}
	if backend.reads != 0 {
		t.Fatal("backend touched without auth")
	}
}
