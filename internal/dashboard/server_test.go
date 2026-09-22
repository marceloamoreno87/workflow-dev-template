package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func validConfig() Config {
	return Config{BindAddr: "127.0.0.1:0", Token: "operator-token-at-least-16"}
}

func TestRejectNonLoopbackBind(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{"0.0.0.0:8080", ":8080", "example.com:80", "not-an-addr", ""} {
		if _, err := NewServer(Config{BindAddr: addr, Token: "operator-token-at-least-16"}); err == nil {
			t.Fatalf("expected rejection for %q, got none", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080", "localhost:8080"} {
		if _, err := NewServer(Config{BindAddr: addr, Token: "operator-token-at-least-16"}); err != nil {
			t.Fatalf("expected acceptance for %q: %v", addr, err)
		}
	}
}

func TestAuthAndSecurityHeaders(t *testing.T) {
	srv, err := NewServer(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	res, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
	req, _ := http.NewRequest("GET", server.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", res.StatusCode)
	}
	req, _ = http.NewRequest("GET", server.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if csp := res.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("weak CSP: %q", csp)
	}
}

func TestOriginCheckOnMutations(t *testing.T) {
	srv, err := NewServer(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	post := func(origin string) int {
		req, _ := http.NewRequest("POST", server.URL+"/api/commands", strings.NewReader(`{}`))
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
	host := strings.TrimPrefix(server.URL, "http://")
	if got := post("http://" + host); got == http.StatusForbidden {
		t.Fatal("same-origin POST was rejected")
	}
	if got := post(""); got != http.StatusForbidden {
		t.Fatalf("missing origin should be 403, got %d", got)
	}
	if got := post("http://evil.example"); got != http.StatusForbidden {
		t.Fatalf("foreign origin should be 403, got %d", got)
	}
}
