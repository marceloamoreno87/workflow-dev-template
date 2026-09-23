// internal/daemon/loops.go
package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marceloamoreno87/workflow-dev-template/internal/loop"
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

const loopsFileName = "daemon-loops.json"

const maxLoopRecords = 100000

type LoopRecord struct {
	Loop        loop.LoopState
	Title       string
	ProjectID   string
	ProductDone bool
}

func (d *Daemon) loopsPath() string {
	return filepath.Join(harnessDir(d.cfg.WorkspaceRoot), loopsFileName)
}

func (d *Daemon) loadLoops() error {
	raw, err := os.ReadFile(d.loopsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(raw) > 4*1024*1024 {
		return fmt.Errorf("%w: loops file too large", ErrDaemon)
	}
	var records map[string]LoopRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return fmt.Errorf("%w: corrupt loops file", ErrDaemon)
	}
	if len(records) > maxLoopRecords {
		return fmt.Errorf("%w: too many loop records", ErrDaemon)
	}
	converted := make(map[workflow.WorkItemID]LoopRecord, len(records))
	for id, rec := range records {
		if id == "" {
			return fmt.Errorf("%w: empty loop id", ErrDaemon)
		}
		converted[workflow.WorkItemID(id)] = rec
	}
	d.loops = converted
	return nil
}

func (d *Daemon) persistLoops() error {
	raw, err := json.Marshal(d.loops)
	if err != nil {
		return err
	}
	return os.WriteFile(d.loopsPath(), raw, 0o644)
}

func (d *Daemon) saveLoop(id workflow.WorkItemID, rec LoopRecord) error {
	if d.loops == nil {
		d.loops = map[workflow.WorkItemID]LoopRecord{}
	}
	d.loops[id] = rec
	return d.persistLoops()
}

func (d *Daemon) LoopState(id workflow.WorkItemID) (loop.LoopState, bool) {
	rec, ok := d.loops[id]
	return rec.Loop, ok
}
