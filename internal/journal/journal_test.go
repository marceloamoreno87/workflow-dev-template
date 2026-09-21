package journal

import (
	"errors"
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

func TestApplyRejectsStaleVersionWithoutNewEvents(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.Apply(time.Unix(1, 0).UTC(), workflow.Command{ID: "c1", AggregateID: "repo#1", ExpectedVersion: 0, ActorID: "operator", Type: workflow.CommandSubmitWork}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(time.Unix(2, 0).UTC(), workflow.Command{ID: "c2", AggregateID: "repo#1", ExpectedVersion: 1, ActorID: "operator", Type: workflow.CommandBeginTriage}); err != nil {
		t.Fatal(err)
	}

	_, err = s.Apply(time.Unix(3, 0).UTC(), workflow.Command{ID: "c3", AggregateID: "repo#1", ExpectedVersion: 1, ActorID: "operator", Type: workflow.CommandAuthorizeWork})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Apply() error = %v, want ErrConflict", err)
	}

	got, err := s.Load("repo#1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != workflow.StateTriage || got.Version != 2 {
		t.Fatalf("unexpected work item: %#v", got)
	}
}

func TestLoadRebuildsStateAndReportsMissing(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.Load("missing#1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}

	if _, err := s.Apply(time.Unix(1, 0).UTC(), workflow.Command{ID: "c1", AggregateID: "repo#9", ExpectedVersion: 0, ActorID: "operator", Type: workflow.CommandSubmitWork}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("repo#9")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "repo#9" || got.State != workflow.StateInbox || got.Version != 1 {
		t.Fatalf("unexpected work item: %#v", got)
	}
}

func TestLeaseAcquireRenewRelease(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Unix(100, 0).UTC()
	ok, err := s.AcquireLease("repo#1", "exec-a", time.Minute, now)
	if err != nil || !ok {
		t.Fatalf("AcquireLease() = %v, %v; want true, nil", ok, err)
	}
	ok, err = s.AcquireLease("repo#1", "exec-b", time.Minute, now)
	if err != nil || ok {
		t.Fatalf("second AcquireLease() = %v, %v; want false, nil", ok, err)
	}
	ok, err = s.RenewLease("repo#1", "exec-b", time.Minute, now)
	if err != nil || ok {
		t.Fatalf("foreign RenewLease() = %v, %v; want false, nil", ok, err)
	}
	ok, err = s.RenewLease("repo#1", "exec-a", time.Minute, now)
	if err != nil || !ok {
		t.Fatalf("RenewLease() = %v, %v; want true, nil", ok, err)
	}
	ok, err = s.ReleaseLease("repo#1", "exec-b")
	if err != nil || ok {
		t.Fatalf("foreign ReleaseLease() = %v, %v; want false, nil", ok, err)
	}
	ok, err = s.ReleaseLease("repo#1", "exec-a")
	if err != nil || !ok {
		t.Fatalf("ReleaseLease() = %v, %v; want true, nil", ok, err)
	}
	ok, err = s.AcquireLease("repo#1", "exec-b", time.Minute, now)
	if err != nil || !ok {
		t.Fatalf("re-acquire = %v, %v; want true, nil", ok, err)
	}
}

func TestExpiredLeaseIsReacquirable(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.Unix(200, 0).UTC()
	if _, err := s.AcquireLease("repo#2", "exec-a", time.Minute, now); err != nil {
		t.Fatal(err)
	}
	ok, err := s.AcquireLease("repo#2", "exec-b", time.Minute, now.Add(2*time.Minute))
	if err != nil || !ok {
		t.Fatalf("expired AcquireLease() = %v, %v; want true, nil", ok, err)
	}
}
