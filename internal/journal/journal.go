// internal/journal/journal.go
package journal

import (
	"database/sql"
	"errors"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS commands (
	command_id TEXT PRIMARY KEY,
	aggregate_id TEXT NOT NULL,
	expected_version INTEGER NOT NULL,
	actor_id TEXT NOT NULL,
	type TEXT NOT NULL,
	reason TEXT NOT NULL,
	received_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
	aggregate_id TEXT NOT NULL,
	version INTEGER NOT NULL,
	command_id TEXT NOT NULL,
	actor_id TEXT NOT NULL,
	type TEXT NOT NULL,
	from_state TEXT NOT NULL,
	to_state TEXT NOT NULL,
	reason TEXT NOT NULL,
	at TEXT NOT NULL,
	PRIMARY KEY (aggregate_id, version)
);
CREATE INDEX IF NOT EXISTS idx_events_command ON events(command_id);
CREATE TABLE IF NOT EXISTS leases (
	resource TEXT PRIMARY KEY,
	holder TEXT NOT NULL,
	expires_at TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ','now'));
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

var ErrConflict = errors.New("stale expected version")

var ErrNotFound = errors.New("work item not found")

func (s *Store) Load(id workflow.WorkItemID) (workflow.WorkItem, error) {
	rows, err := s.db.Query(`SELECT aggregate_id, version, command_id, actor_id, type, from_state, to_state, reason, at FROM events WHERE aggregate_id=? ORDER BY version`, string(id))
	if err != nil {
		return workflow.WorkItem{}, err
	}
	defer rows.Close()
	events, err := scanEvents(rows)
	if err != nil {
		return workflow.WorkItem{}, err
	}
	if len(events) == 0 {
		return workflow.WorkItem{}, ErrNotFound
	}
	return workflow.Fold(events)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

func (s *Store) Apply(now time.Time, cmd workflow.Command) ([]workflow.Event, error) {
	now = now.UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var commandID, aggregateID, actorID, commandType, reason, receivedAt string
	var expectedVersion uint64
	err = tx.QueryRow(`SELECT command_id, aggregate_id, expected_version, actor_id, type, reason, received_at FROM commands WHERE command_id=?`, string(cmd.ID)).Scan(&commandID, &aggregateID, &expectedVersion, &actorID, &commandType, &reason, &receivedAt)
	if err == nil {
		rows, err := tx.Query(`SELECT aggregate_id, version, command_id, actor_id, type, from_state, to_state, reason, at FROM events WHERE command_id=? ORDER BY version`, string(cmd.ID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		events, err := scanEvents(rows)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return events, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	current, err := loadWorkItemTx(tx, cmd.AggregateID)
	if err != nil {
		return nil, err
	}
	if cmd.ExpectedVersion != current.Version {
		return nil, ErrConflict
	}
	events, err := (workflow.Workflow{}).Handle(now, current, cmd)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO commands(command_id, aggregate_id, expected_version, actor_id, type, reason, received_at) VALUES(?,?,?,?,?,?,?)`, string(cmd.ID), string(cmd.AggregateID), uint64(cmd.ExpectedVersion), string(cmd.ActorID), string(cmd.Type), cmd.Reason, formatTime(now)); err != nil {
		return nil, err
	}
	for _, e := range events {
		if _, err := tx.Exec(`INSERT INTO events(aggregate_id, version, command_id, actor_id, type, from_state, to_state, reason, at) VALUES(?,?,?,?,?,?,?,?,?)`, string(e.AggregateID), uint64(e.Version), string(e.CommandID), string(e.ActorID), string(e.Type), string(e.From), string(e.To), e.Reason, formatTime(e.At)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return events, nil
}

func scanEvents(rows *sql.Rows) ([]workflow.Event, error) {
	var out []workflow.Event
	for rows.Next() {
		var aggregateID, commandID, actorID, eventType, fromState, toState, reason, at string
		var version uint64
		if err := rows.Scan(&aggregateID, &version, &commandID, &actorID, &eventType, &fromState, &toState, &reason, &at); err != nil {
			return nil, err
		}
		parsed, err := parseTime(at)
		if err != nil {
			return nil, err
		}
		out = append(out, workflow.Event{
			AggregateID: workflow.WorkItemID(aggregateID),
			Version:     workflow.Version(version),
			CommandID:   workflow.CommandID(commandID),
			ActorID:     workflow.ActorID(actorID),
			Type:        workflow.EventType(eventType),
			From:        workflow.State(fromState),
			To:          workflow.State(toState),
			Reason:      reason,
			At:          parsed,
		})
	}
	return out, rows.Err()
}

func loadWorkItemTx(tx *sql.Tx, id workflow.WorkItemID) (workflow.WorkItem, error) {
	rows, err := tx.Query(`SELECT aggregate_id, version, command_id, actor_id, type, from_state, to_state, reason, at FROM events WHERE aggregate_id=? ORDER BY version`, string(id))
	if err != nil {
		return workflow.WorkItem{}, err
	}
	defer rows.Close()
	events, err := scanEvents(rows)
	if err != nil {
		return workflow.WorkItem{}, err
	}
	if len(events) == 0 {
		return workflow.WorkItem{}, nil
	}
	return workflow.Fold(events)
}

func (s *Store) AcquireLease(resource, holder string, ttl time.Duration, now time.Time) (bool, error) {
	now = now.UTC()
	expires := formatTime(now.Add(ttl))
	res, err := s.db.Exec(`INSERT INTO leases(resource, holder, expires_at) VALUES(?,?,?) ON CONFLICT(resource) DO UPDATE SET holder=excluded.holder, expires_at=excluded.expires_at WHERE leases.expires_at <= ?`, resource, holder, expires, formatTime(now))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (s *Store) RenewLease(resource, holder string, ttl time.Duration, now time.Time) (bool, error) {
	now = now.UTC()
	res, err := s.db.Exec(`UPDATE leases SET expires_at=? WHERE resource=? AND holder=? AND expires_at > ?`, formatTime(now.Add(ttl)), resource, holder, formatTime(now))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (s *Store) ReleaseLease(resource, holder string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM leases WHERE resource=? AND holder=?`, resource, holder)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}
