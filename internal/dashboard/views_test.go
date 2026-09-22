package dashboard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func testServer(t *testing.T, items ...Item) (*Server, *httptest.Server) {
	t.Helper()

	srv, err := NewServer(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		srv.Items().Upsert(item)
	}
	server := httptest.NewServer(srv.Handler())
	t.Cleanup(server.Close)
	return srv, server
}

func authedGet(t *testing.T, url string) (int, string) {
	t.Helper()

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer operator-token-at-least-16")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(raw)
}

func TestIndexListsItemsSorted(t *testing.T) {
	_, server := testServer(t,
		Item{ID: "owner/repo#9", State: workflow.StateDone, Version: 5},
		Item{ID: "owner/repo#10", State: workflow.StateInbox, Version: 1},
	)
	code, body := authedGet(t, server.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	first := strings.Index(body, "owner/repo#10")
	second := strings.Index(body, "owner/repo#9")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("items not sorted:\n%s", body)
	}
	if !strings.Contains(body, "inbox") || !strings.Contains(body, "done") {
		t.Fatalf("states missing:\n%s", body)
	}
}

func TestDetailRendersAndEscapes(t *testing.T) {
	_, server := testServer(t, Item{ID: "<script>alert(1)</script>", State: workflow.StateTriage, Version: 2})
	code, body := authedGet(t, server.URL+"/items/"+url.PathEscape("<script>alert(1)</script>"))
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("unescaped content:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("escaped content missing:\n%s", body)
	}
	code, _ = authedGet(t, server.URL+"/items/owner%2Frepo%23123")
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}
