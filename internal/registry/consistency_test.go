package registry

import (
	"errors"
	"testing"
)

const exampleGitmodules = `[submodule "projects/example"]
	path = projects/example
	url = git@github.com:owner/repo.git
`

func TestConsistencyAcceptsMatchingWorkspace(t *testing.T) {
	t.Parallel()

	r, err := ParseRegistry([]byte(validRegistry))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseProjectManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckConsistency(r, map[string]ProjectManifest{"example": m}, exampleGitmodules); err != nil {
		t.Fatal(err)
	}
}

func TestConsistencyRejectsMismatches(t *testing.T) {
	t.Parallel()

	r, err := ParseRegistry([]byte(validRegistry))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseProjectManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("missing manifest", func(t *testing.T) {
		if err := CheckConsistency(r, map[string]ProjectManifest{}, exampleGitmodules); err == nil {
			t.Fatal("expected missing manifest error")
		}
	})

	t.Run("id mismatch", func(t *testing.T) {
		other := m
		other.Project.ID = "renamed"
		if err := CheckConsistency(r, map[string]ProjectManifest{"example": other}, exampleGitmodules); err == nil {
			t.Fatal("expected id mismatch error")
		}
	})

	t.Run("repository mismatch", func(t *testing.T) {
		other := m
		other.Project.Repository = "owner/other"
		err := CheckConsistency(r, map[string]ProjectManifest{"example": other}, exampleGitmodules)
		if err == nil || !errors.Is(err, ErrManifest) {
			t.Fatalf("expected manifest mismatch, got %v", err)
		}
	})

	t.Run("missing submodule", func(t *testing.T) {
		if err := CheckConsistency(r, map[string]ProjectManifest{"example": m}, ""); err == nil {
			t.Fatal("expected submodule error")
		}
	})
}
