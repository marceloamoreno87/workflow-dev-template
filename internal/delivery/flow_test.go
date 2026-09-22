package delivery

import (
	"reflect"
	"testing"
)

func TestSeparateFactsFlow(t *testing.T) {
	t.Parallel()

	commits := ownCommits()
	if err := CanReorganize(commits, false); err != nil {
		t.Fatal(err)
	}
	pr := validPR()
	pr.Commits = commits
	if err := CanMerge(pr); err != nil {
		t.Fatal(err)
	}
	release, err := CreateRelease("v0.1.0", pr.Head, validArtifacts())
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyRelease(release); err != nil {
		t.Fatal(err)
	}
	// Approval alone merges nothing: an unapproved PR with green checks still fails.
	unapproved := pr
	unapproved.ApprovedByOperator = false
	if err := CanMerge(unapproved); err == nil {
		t.Fatal("approval and merge are not separate")
	}
	// A stripped Release fails closed.
	stripped := release
	stripped.Artifacts = nil
	if err := VerifyRelease(stripped); err == nil {
		t.Fatal("stripped release verified")
	}
	// A frozen range stays frozen even for its owner.
	if err := CanReorganize(commits, true); err == nil {
		t.Fatal("post-creation rewrite allowed")
	}
	if err := validEvidence().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMergeEvaluatesWithoutMutating(t *testing.T) {
	t.Parallel()

	pr := validPR()
	before := pr
	if err := CanMerge(pr); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pr, before) {
		t.Fatalf("CanMerge mutated its input:\nbefore: %#v\nafter:  %#v", before, pr)
	}
}
