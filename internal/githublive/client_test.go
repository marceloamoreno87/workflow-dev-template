package githublive

import (
	"testing"
)

func TestRejectBadClients(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		baseURL string
		token   string
	}{
		{name: "empty url", baseURL: "", token: "tok"},
		{name: "no host", baseURL: "https://", token: "tok"},
		{name: "userinfo", baseURL: "https://user:pass@api.example", token: "tok"},
		{name: "empty token", baseURL: "https://api.example", token: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(tc.baseURL, tc.token); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestStaticToken(t *testing.T) {
	t.Parallel()

	token, err := StaticToken("tok").Token(t.Context())
	if err != nil || token != "tok" {
		t.Fatalf("unexpected token: %q %v", token, err)
	}
	if _, err := StaticToken("").Token(t.Context()); err == nil {
		t.Fatal("expected empty rejection, got none")
	}
}
