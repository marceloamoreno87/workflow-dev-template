package journal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

func TestOpenCreatesSchema(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "journal.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, table := range []string{"schema_migrations", "commands", "events", "leases"} {
		var name string
		err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %q missing: %v", table, err)
		}
	}

	var mode string
	if err := s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func TestApplyIsIdempotentByCommandID(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	cmd := workflow.Command{ID: "c1", AggregateID: "repo#1", ExpectedVersion: 0, ActorID: "operator", Type: workflow.CommandSubmitWork}
	first, err := s.Apply(time.Unix(1, 0).UTC(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Version != 1 {
		t.Fatalf("unexpected first events: %#v", first)
	}

	second, err := s.Apply(time.Unix(2, 0).UTC(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Version != 1 || second[0].CommandID != "c1" {
		t.Fatalf("unexpected replayed events: %#v", second)
	}

	count := 0
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE aggregate_id='repo#1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("event count = %d, want 1", count)
	}
}
