package delivery

import (
	"strings"
	"testing"
)

func validArtifacts() []Artifact {
	return []Artifact{
		{Name: "harness-linux-amd64", Digest: "sha256:" + strings.Repeat("a", 64)},
	}
}

func TestCreateRelease(t *testing.T) {
	t.Parallel()

	r, err := CreateRelease("v1.2.3", strings.Repeat("f", 40), validArtifacts())
	if err != nil {
		t.Fatal(err)
	}
	if r.Tag != "v1.2.3" || len(r.Artifacts) != 1 {
		t.Fatalf("unexpected release: %#v", r)
	}
	if err := VerifyRelease(r); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadReleases(t *testing.T) {
	t.Parallel()

	sha := strings.Repeat("f", 40)
	for _, tc := range []struct {
		name      string
		tag       string
		commit    string
		artifacts []Artifact
	}{
		{name: "missing v", tag: "1.2.3", commit: sha, artifacts: validArtifacts()},
		{name: "no patch", tag: "v1.2", commit: sha, artifacts: validArtifacts()},
		{name: "prerelease suffix", tag: "v1.2.3-rc1", commit: sha, artifacts: validArtifacts()},
		{name: "short sha", tag: "v1.2.3", commit: "abc", artifacts: validArtifacts()},
		{name: "uppercase sha", tag: "v1.2.3", commit: strings.Repeat("F", 40), artifacts: validArtifacts()},
		{name: "no artifacts", tag: "v1.2.3", commit: sha, artifacts: nil},
		{name: "empty name", tag: "v1.2.3", commit: sha, artifacts: []Artifact{{Name: "", Digest: "sha256:" + strings.Repeat("a", 64)}}},
		{name: "bad digest", tag: "v1.2.3", commit: sha, artifacts: []Artifact{{Name: "bin", Digest: "md5:abc"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := CreateRelease(tc.tag, tc.commit, tc.artifacts); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestMutatedReleaseFailsVerification(t *testing.T) {
	t.Parallel()

	// Provenance boundary: VerifyRelease re-validates shape, not history.
	// A well-formed swap still verifies; only a shape violation fails closed.
	// Detecting silent swaps against the recorded Release belongs to the
	// journal/registry layer (later increment).
	r, err := CreateRelease("v1.2.3", strings.Repeat("f", 40), validArtifacts())
	if err != nil {
		t.Fatal(err)
	}
	stripped := r
	stripped.Artifacts = nil
	if err := VerifyRelease(stripped); err == nil {
		t.Fatal("expected stripped release to fail verification, got none")
	}
}
