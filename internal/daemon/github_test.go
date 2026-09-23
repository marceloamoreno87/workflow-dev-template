package daemon

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/githublive"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

// pollerAdapter bridges a githublive Poller to the daemon IntakeSource by mapping
// validated issues to intake items. Production wiring (flags, tokens, scheduling)
// arrives with the human-surface increment.
type pollerAdapter struct {
	poller *githublive.Poller
	owner  string
	repo   string
}

func (a *pollerAdapter) Poll(ctx context.Context, since time.Time, limit int) ([]IntakeItem, error) {
	issues, err := a.poller.Poll(ctx, since, limit)
	if err != nil {
		return nil, err
	}
	items := make([]IntakeItem, 0, len(issues))
	for _, issue := range issues {
		items = append(items, IntakeItem{
			ID:    workflow.WorkItemID(fmt.Sprintf("%s/%s#%d", a.owner, a.repo, issue.Number)),
			Title: issue.Title,
		})
	}
	return items, nil
}

func TestPollerDrivesDaemon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":78,"title":"Add health endpoint","body":"x","state":"open","user":{"login":"octocat"},"updated_at":"2026-09-23T00:00:00Z"}]`))
	}))
	defer server.Close()

	client, err := githublive.NewClient(server.URL, "stub-token")
	if err != nil {
		t.Fatal(err)
	}
	poller, err := githublive.NewPoller(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	d, err := Open(testConfig(root), &pollerAdapter{poller: poller, owner: "owner", repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	didWork, err := d.Tick(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !didWork {
		t.Fatal("poller item not submitted")
	}
	item := mustLoad(t, d, "owner/repo#78")
	if item.State != workflow.StateTriage {
		t.Fatalf("poller item not triaged: %#v", item)
	}
}

func TestLiveGitHubIsGated(t *testing.T) {
	if os.Getenv("HARNESS_GITHUB_URL") == "" || os.Getenv("HARNESS_GITHUB_TOKEN") == "" {
		t.Skip("set HARNESS_GITHUB_URL and HARNESS_GITHUB_TOKEN to run against live GitHub")
	}
}
