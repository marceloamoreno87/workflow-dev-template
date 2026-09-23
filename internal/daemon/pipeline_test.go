package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

// authorizeAsHuman applies authorize_work directly to the journal, simulating what
// the Telegram/dashboard surfaces do through the same gatekeeper and journal.
func authorizeAsHuman(t *testing.T, d *Daemon, id workflow.WorkItemID, version workflow.Version) {
	t.Helper()

	if _, err := d.Journal().Apply(time.Now(), workflow.Command{
		ID:              "human-authorize",
		AggregateID:     id,
		ExpectedVersion: version,
		ActorID:         "actor/operator",
		Type:            workflow.CommandAuthorizeWork,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestHappyPathPipeline(t *testing.T) {
	root := fixtureWorkspace(t)
	bin := stubBin(t)
	writeStub(t, bin, "gofmt", "#!/bin/sh\nexit 0\n")
	writeStub(t, bin, "go", "#!/bin/sh\nexit 0\n")
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#123", Title: "Add health endpoint"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := mustLoad(t, d, "owner/repo#123"); got.State != workflow.StateTriage {
		t.Fatalf("expected triage: %#v", got)
	}
	authorizeAsHuman(t, d, "owner/repo#123", 2)

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	afterImplement := mustLoad(t, d, "owner/repo#123")
	if afterImplement.State != workflow.StateReviewing {
		t.Fatalf("expected reviewing after gates: %#v", afterImplement)
	}
	state, ok := d.LoopState("owner/repo#123")
	if !ok || state.Stage != "reviewing" {
		t.Fatalf("loop not reviewing: %#v %v", state, ok)
	}

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	final := mustLoad(t, d, "owner/repo#123")
	if final.State != workflow.StateReviewing {
		t.Fatalf("reviewer must not move workflow past the human gate: %#v", final)
	}
	done, ok := d.LoopState("owner/repo#123")
	if !ok || done.Stage != "done" {
		t.Fatalf("loop not done: %#v %v", done, ok)
	}
	if _, err := os.Stat(filepath.Join(root, ".worktrees", "demo")); !os.IsNotExist(err) {
		t.Fatalf("worktree survives loop done: %v", err)
	}
}

func TestLoopDoneIssuesNoWorkflowCommand(t *testing.T) {
	root := fixtureWorkspace(t)
	bin := stubBin(t)
	writeStub(t, bin, "gofmt", "#!/bin/sh\nexit 0\n")
	writeStub(t, bin, "go", "#!/bin/sh\nexit 0\n")
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#123"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	authorizeAsHuman(t, d, "owner/repo#123", 2)
	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	version := mustLoad(t, d, "owner/repo#123").Version
	quiet, err := d.Tick(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if quiet {
		t.Fatal("finished loop must be quiet")
	}
	if got := mustLoad(t, d, "owner/repo#123").Version; got != version {
		t.Fatalf("finished loop mutated workflow: %d -> %d", version, got)
	}
}

func TestRepeatedGateFailureBlocks(t *testing.T) {
	root := fixtureWorkspace(t)
	bin := stubBin(t)
	writeStub(t, bin, "gofmt", "#!/bin/sh\nexit 0\n")
	writeStub(t, bin, "go", "#!/bin/sh\ncase \"$*\" in *test*) exit 1;; *) exit 0;; esac\n")
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#123"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	authorizeAsHuman(t, d, "owner/repo#123", 2)

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	first := mustLoad(t, d, "owner/repo#123")
	if first.State != workflow.StateImplementing {
		t.Fatalf("expected implementing after novel failure: %#v", first)
	}
	state, _ := d.LoopState("owner/repo#123")
	if state.Fixes != 1 {
		t.Fatalf("expected one fix cycle: %#v", state)
	}

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	blocked := mustLoad(t, d, "owner/repo#123")
	if blocked.State != workflow.StateBlocked {
		t.Fatalf("expected mirrored block: %#v", blocked)
	}
	loopState, _ := d.LoopState("owner/repo#123")
	if loopState.BlockedReason != "repeated failure" {
		t.Fatalf("wrong block reason: %#v", loopState)
	}
	quiet, err := d.Tick(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if quiet {
		t.Fatal("blocked loop must be quiet")
	}
}

func TestRestartMidPipeline(t *testing.T) {
	root := fixtureWorkspace(t)
	bin := stubBin(t)
	writeStub(t, bin, "gofmt", "#!/bin/sh\nexit 0\n")
	writeStub(t, bin, "go", "#!/bin/sh\nexit 0\n")
	d, err := Open(testConfig(root), &fakeSource{items: []IntakeItem{{ID: "owner/repo#123"}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	authorizeAsHuman(t, d, "owner/repo#123", 2)
	if _, err := d.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	before := mustLoad(t, d, "owner/repo#123")
	if before.State != workflow.StateReviewing {
		t.Fatalf("expected reviewing before restart: %#v", before)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(testConfig(root), &fakeSource{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	if _, err := reopened.Tick(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	after := mustLoad(t, reopened, "owner/repo#123")
	if after.Version != before.Version {
		t.Fatalf("restart duplicated workflow commands: %d -> %d", before.Version, after.Version)
	}
	// The reviewer leg reruns (loop was still reviewing on disk) and completes
	// the loop; the workflow itself must not move past the human gate.
	if after.State != workflow.StateReviewing {
		t.Fatalf("restart moved workflow past the human gate: %#v", after)
	}
	done, ok := reopened.LoopState("owner/repo#123")
	if !ok || done.Stage != "done" {
		t.Fatalf("loop not resumed to done: %#v %v", done, ok)
	}
}
