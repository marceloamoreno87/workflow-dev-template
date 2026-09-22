package delivery

import (
	"strings"
	"testing"
)

func ownCommits() []Commit {
	return []Commit{
		{SHA: strings.Repeat("a", 40), Author: "actor/automation"},
		{SHA: strings.Repeat("b", 40), Author: "actor/automation"},
	}
}

func TestReorganizeOwnCommitsBeforeCreation(t *testing.T) {
	t.Parallel()

	if err := CanReorganize(ownCommits(), false); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadReorganizations(t *testing.T) {
	t.Parallel()

	thirdParty := append(ownCommits(), Commit{SHA: strings.Repeat("c", 40), Author: "actor/contributor", ThirdParty: true})
	for _, tc := range []struct {
		name      string
		commits   []Commit
		prCreated bool
	}{
		{name: "after creation", commits: ownCommits(), prCreated: true},
		{name: "third party before creation", commits: thirdParty, prCreated: false},
		{name: "third party after creation", commits: thirdParty, prCreated: true},
		{name: "empty range", commits: nil, prCreated: false},
		{name: "bad sha", commits: []Commit{{SHA: "xyz", Author: "actor/automation"}}, prCreated: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CanReorganize(tc.commits, tc.prCreated); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func validPR() PullRequest {
	return PullRequest{
		Base:               "main",
		Head:               strings.Repeat("d", 40),
		Commits:            ownCommits(),
		Created:            true,
		ApprovedByOperator: true,
		ChecksPassed:       true,
		MergeMethod:        "merge",
	}
}

func TestMergeEligibility(t *testing.T) {
	t.Parallel()

	if err := CanMerge(validPR()); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadMerges(t *testing.T) {
	t.Parallel()

	mk := func(mut func(*PullRequest)) PullRequest {
		pr := validPR()
		mut(&pr)
		return pr
	}
	for _, tc := range []struct {
		name string
		pr   PullRequest
	}{
		{name: "not created", pr: mk(func(p *PullRequest) { p.Created = false })},
		{name: "not approved", pr: mk(func(p *PullRequest) { p.ApprovedByOperator = false })},
		{name: "checks red", pr: mk(func(p *PullRequest) { p.ChecksPassed = false })},
		{name: "squash loses history", pr: mk(func(p *PullRequest) { p.MergeMethod = "squash" })},
		{name: "rebase rewrites", pr: mk(func(p *PullRequest) { p.MergeMethod = "rebase" })},
		{name: "empty base", pr: mk(func(p *PullRequest) { p.Base = "" })},
		{name: "bad head", pr: mk(func(p *PullRequest) { p.Head = "main" })},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CanMerge(tc.pr); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestThirdPartyCommitsMergeUnrewritten(t *testing.T) {
	t.Parallel()

	pr := validPR()
	pr.Commits = append(pr.Commits, Commit{SHA: strings.Repeat("e", 40), Author: "actor/contributor", ThirdParty: true})
	if err := CanMerge(pr); err != nil {
		t.Fatalf("third-party commits are preserved, not rejected: %v", err)
	}
	if err := CanReorganize(pr.Commits, false); err == nil {
		t.Fatal("range with third-party commits must never reorganize")
	}
}
