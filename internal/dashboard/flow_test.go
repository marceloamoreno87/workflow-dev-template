package dashboard

import (
	"net/http"
	"strings"
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestGuardedFlow(t *testing.T) {
	_, server := testServer(t, Item{ID: "owner/repo#123", State: workflow.StateReviewing, Version: 7})
	host := strings.TrimPrefix(server.URL, "http://")

	get := func(path, token string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest("GET", server.URL+path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res
	}
	if res := get("/", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous index: %d", res.StatusCode)
	}
	res := get("/", "operator-token-at-least-16")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("authed index: %d", res.StatusCode)
	}
	if csp := res.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("CSP missing: %q", csp)
	}
	res.Body.Close()

	post := func(body, origin string) int {
		t.Helper()
		req, _ := http.NewRequest("POST", server.URL+"/api/commands", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if got := post(`{"aggregateId":"owner/repo#123","expectedVersion":7,"type":"approve_pr","reason":""}`, "http://"+host); got != http.StatusNotImplemented {
		t.Fatalf("same-origin approve without applier: %d", got)
	}
	if got := post(`{"aggregateId":"owner/repo#123","expectedVersion":7,"type":"approve_pr","reason":""}`, "http://evil.example"); got != http.StatusForbidden {
		t.Fatalf("foreign origin: %d", got)
	}
}
