// internal/daemon/daemon.go
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/dashboard"
	"github.com/marceloamoreno87/workflow-dev-template/internal/gatekeeper"
	"github.com/marceloamoreno87/workflow-dev-template/internal/journal"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

type IntakeItem struct {
	ID    workflow.WorkItemID
	Title string
}

type IntakeSource interface {
	Poll(since time.Time, limit int) ([]IntakeItem, error)
}

type emptySource struct{}

func EmptySource() IntakeSource { return emptySource{} }

func (emptySource) Poll(time.Time, int) ([]IntakeItem, error) { return nil, nil }

const knownFileName = "daemon-known.json"

const maxKnownItems = 100000

type Daemon struct {
	cfg      Config
	db       *journal.Store
	dash     *dashboard.Server
	source   IntakeSource
	lastPoll time.Time
	known    map[workflow.WorkItemID]bool
	loops    map[workflow.WorkItemID]LoopRecord
}

func harnessDir(root string) string {
	return filepath.Join(root, ".harness")
}

func Open(cfg Config, source IntakeSource) (*Daemon, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("%w: intake source required", ErrDaemon)
	}
	if err := os.MkdirAll(harnessDir(cfg.WorkspaceRoot), 0o755); err != nil {
		return nil, err
	}
	db, err := journal.Open(filepath.Join(harnessDir(cfg.WorkspaceRoot), "harness.db"))
	if err != nil {
		return nil, err
	}
	d := &Daemon{cfg: cfg, db: db, source: source, known: map[workflow.WorkItemID]bool{}, loops: map[workflow.WorkItemID]LoopRecord{}}
	if err := d.loadKnown(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := d.loadLoops(); err != nil {
		_ = db.Close()
		return nil, err
	}
	dash, err := dashboard.NewServer(dashboard.Config{BindAddr: cfg.BindAddr, Token: cfg.Token})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	d.dash = dash
	d.feedDashboard()
	return d, nil
}

func (d *Daemon) Close() error {
	return d.db.Close()
}

func (d *Daemon) Journal() *journal.Store {
	return d.db
}

func (d *Daemon) Dashboard() *dashboard.Server {
	return d.dash
}

func (d *Daemon) knownPath() string {
	return filepath.Join(harnessDir(d.cfg.WorkspaceRoot), knownFileName)
}

func (d *Daemon) loadKnown() error {
	raw, err := os.ReadFile(d.knownPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(raw) > 4*1024*1024 {
		return fmt.Errorf("%w: known file too large", ErrDaemon)
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return fmt.Errorf("%w: corrupt known file", ErrDaemon)
	}
	if len(ids) > maxKnownItems {
		return fmt.Errorf("%w: too many known items", ErrDaemon)
	}
	for _, id := range ids {
		if id == "" {
			return fmt.Errorf("%w: empty known id", ErrDaemon)
		}
		d.known[workflow.WorkItemID(id)] = true
	}
	return nil
}

func (d *Daemon) persistKnown() error {
	ids := make([]string, 0, len(d.known))
	for id := range d.known {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return os.WriteFile(d.knownPath(), raw, 0o644)
}

func (d *Daemon) feedDashboard() {
	for id := range d.known {
		item, err := d.db.Load(id)
		if err != nil {
			continue
		}
		d.dash.Items().Upsert(dashboard.Item{ID: item.ID, State: item.State, Version: item.Version})
	}
}

func (d *Daemon) Tick(ctx context.Context, now time.Time) (bool, error) {
	items, err := d.source.Poll(d.lastPoll, 100)
	if err != nil {
		return false, err
	}
	didWork := false
	for n, intake := range items {
		if intake.ID == "" {
			return false, fmt.Errorf("%w: intake without id", ErrDaemon)
		}
		if d.known[intake.ID] {
			continue
		}
		if _, err := d.db.Load(intake.ID); err == nil {
			d.known[intake.ID] = true
			continue
		} else if !errors.Is(err, journal.ErrNotFound) {
			return false, err
		}
		reason := strings.TrimSpace(intake.Title)
		if reason == "" {
			reason = "github intake"
		}
		cmd := workflow.Command{
			ID:              workflow.CommandID(fmt.Sprintf("daemon-intake-%d-%d", now.UnixNano(), n)),
			AggregateID:     intake.ID,
			ExpectedVersion: 0,
			ActorID:         "actor/automation",
			Type:            workflow.CommandSubmitWork,
			Reason:          reason,
		}
		decision := (gatekeeper.Policy{}).Decide(gatekeeper.Context{
			Actor: gatekeeper.ActorAutomation, Profile: gatekeeper.ProfileStandard, ProjectMinimum: gatekeeper.ProfilePrototype,
		}, cmd)
		if !decision.Allowed {
			return false, fmt.Errorf("%w: intake denied (%s)", ErrDaemon, decision.Code)
		}
		if _, err := d.db.Apply(now, cmd); err != nil {
			return false, err
		}
		d.known[intake.ID] = true
		didWork = true
	}
	if err := d.persistKnown(); err != nil {
		return false, err
	}
	d.lastPoll = now
	for _, id := range sortedIDs(d.known) {
		worked, err := d.advanceItem(ctx, id, now)
		didWork = didWork || worked
		if err != nil {
			d.feedDashboard()
			return didWork, err
		}
	}
	d.feedDashboard()
	return didWork, nil
}
