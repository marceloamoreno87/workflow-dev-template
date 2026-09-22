package workspace

import (
	"path/filepath"
	"testing"
)

func TestWorktreeDirConfinesProject(t *testing.T) {
	t.Parallel()

	dir, err := WorktreeDir("/ws", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join("/ws", ".worktrees", "demo") {
		t.Fatalf("unexpected dir: %q", dir)
	}
}

func TestRejectBadRequests(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		root          string
		projectID     string
		submodulePath string
	}{
		{name: "relative root", root: "ws", projectID: "demo", submodulePath: "/ws/projects/demo"},
		{name: "empty id", root: "/ws", projectID: "", submodulePath: "/ws/projects/demo"},
		{name: "slash id", root: "/ws", projectID: "../evil", submodulePath: "/ws/projects/evil"},
		{name: "space id", root: "/ws", projectID: "de mo", submodulePath: "/ws/projects/de mo"},
		{name: "escape submodule", root: "/ws", projectID: "demo", submodulePath: "/ws/../evil"},
		{name: "outside submodule", root: "/ws", projectID: "demo", submodulePath: "/other/demo"},
		{name: "relative submodule", root: "/ws", projectID: "demo", submodulePath: "projects/demo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRequest(tc.root, tc.projectID, tc.submodulePath); err == nil {
				t.Fatal("expected rejection, got none")
			}
			if _, err := WorktreeDir(tc.root, tc.projectID); tc.name == "slash id" || tc.name == "space id" || tc.name == "empty id" {
				if err == nil {
					t.Fatal("expected WorktreeDir rejection, got none")
				}
			}
		})
	}
}
