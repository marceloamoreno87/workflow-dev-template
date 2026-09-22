package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteGitConfigDisablesHooksAndHelpers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prepared, err := WriteGitConfig(filepath.Join(dir, ".worktrees", "demo"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(prepared.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(raw)
	if !strings.Contains(cfg, "hooksPath") || !strings.Contains(cfg, prepared.HooksDir) {
		t.Fatalf("hooksPath missing: %q", cfg)
	}
	if !strings.Contains(cfg, "[credential]") {
		t.Fatalf("credential section missing: %q", cfg)
	}
	info, err := os.Stat(prepared.HooksDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("hooks dir missing: %v", err)
	}
	entries, err := os.ReadDir(prepared.HooksDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("hooks dir must be empty: %v %v", entries, err)
	}
}

func TestWriteGitConfigRejectsBadDir(t *testing.T) {
	t.Parallel()

	if _, err := WriteGitConfig("relative/dir"); err == nil {
		t.Fatal("expected rejection of relative dir, got none")
	}
}
