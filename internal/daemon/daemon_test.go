package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func ctx() context.Context {
	return context.Background()
}

type fakeSource struct {
	items []IntakeItem
	err   error
}

func (f *fakeSource) Poll(since time.Time, limit int) ([]IntakeItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func testConfig(root string) Config {
	return Config{
		WorkspaceRoot: root,
		PollInterval:  5 * time.Second,
		BindAddr:      "127.0.0.1:0",
		Token:         "operator-token-at-least-16",
		RequiredRoles: []string{"product", "implementer", "reviewer"},
		BudgetUSD:     10,
		MaxDuration:   2 * time.Hour,
	}
}

func TestTickSubmitsAndTriages(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{
		{ID: "owner/repo#1", Title: "First"},
		{ID: "owner/repo#2", Title: "Second"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	now := time.Now()
	didWork, err := d.Tick(ctx(), now)
	if err != nil {
		t.Fatal(err)
	}
	if !didWork {
		t.Fatal("expected work, got none")
	}
	item, err := d.Journal().Load("owner/repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if item.State != workflow.StateTriage || item.Version != 2 {
		t.Fatalf("unexpected state: %#v", item)
	}
	again, err := d.Tick(ctx(), now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("re-poll must be a no-op")
	}
	stable, err := d.Journal().Load("owner/repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if stable.Version != 2 {
		t.Fatalf("duplicate submit: %#v", stable)
	}
}

func TestTickPicksUpSeededItems(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#9"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	seeded, err := d.Journal().Apply(time.Now(), workflow.Command{
		ID:              "seed-1",
		AggregateID:     "owner/repo#9",
		ExpectedVersion: 0,
		ActorID:         "actor/automation",
		Type:            workflow.CommandSubmitWork,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seeded) != 1 {
		t.Fatalf("seed failed: %#v", seeded)
	}

	didWork, err := d.Tick(ctx(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !didWork {
		t.Fatal("reported journaled item should advance to triage, not resubmit")
	}
	advanced, err := d.Journal().Load("owner/repo#9")
	if err != nil {
		t.Fatal(err)
	}
	if advanced.State != workflow.StateTriage || advanced.Version != 2 {
		t.Fatalf("seeded item not triaged exactly once: %#v", advanced)
	}
}

func TestTickFeedsDashboard(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#9"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Tick(ctx(), time.Now()); err != nil {
		t.Fatal(err)
	}
	rows := d.Dashboard().Items().List()
	if len(rows) != 1 || rows[0].ID != "owner/repo#9" || rows[0].State != workflow.StateTriage {
		t.Fatalf("dashboard not fed: %#v", rows)
	}
}

func TestRestartRecoversCursor(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Tick(ctx(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#2"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	didWork, err := reopened.Tick(ctx(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !didWork {
		t.Fatal("reopened daemon should process the new item")
	}
	resumed, err := reopened.Journal().Load("owner/repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if resumed.State != workflow.StateTriage || resumed.Version != 2 {
		t.Fatalf("known item resubmitted after restart: %#v", resumed)
	}
	fresh, err := reopened.Journal().Load("owner/repo#2")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.State != workflow.StateTriage {
		t.Fatalf("new item not processed after restart: %#v", fresh)
	}
	quiet, err := reopened.Tick(ctx(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if quiet {
		t.Fatal("triaged item must wait for the human gate")
	}
	if rows := reopened.Dashboard().Items().List(); len(rows) != 2 {
		t.Fatalf("dashboard not refed after restart: %#v", rows)
	}
}

func TestTickPropagatesSourceErrors(t *testing.T) {
	root := t.TempDir()
	d, err := Open(testConfig(root), &fakeSource{err: errors.New("source down")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Tick(ctx(), time.Now()); err == nil {
		t.Fatal("expected source error, got none")
	}
}
