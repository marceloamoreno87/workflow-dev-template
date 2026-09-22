package knowledge

import (
	"strings"
	"testing"
)

func validProposal() Proposal {
	return Proposal{
		ConceptID: "deploy-freeze",
		Title:     "Deploy Freeze",
		Body:      "Freeze production deploys on Fridays and holidays.",
		Sources:   []string{"owner/repo#123"},
		Reason:    "holidays added by operator review",
		Version:   3,
	}
}

func humanVerification() Verification {
	return Verification{Method: "human", Evidence: "operator approved in review"}
}

func TestApplyProposal(t *testing.T) {
	t.Parallel()

	bundle := Bundle{Concepts: map[string]Concept{
		"deploy-freeze": {ID: "deploy-freeze", Title: "Deploy Freeze", Version: 2, Status: "active", Body: "old", Path: "deploy-freeze.md"},
	}}
	if err := Apply(&bundle, validProposal(), humanVerification()); err != nil {
		t.Fatal(err)
	}
	if bundle.Concepts["deploy-freeze"].Version != 3 {
		t.Fatalf("version not bumped: %#v", bundle.Concepts["deploy-freeze"])
	}
}

func TestApplyNewConcept(t *testing.T) {
	t.Parallel()

	bundle := Bundle{Concepts: map[string]Concept{}}
	proposal := validProposal()
	proposal.ConceptID = "on-call"
	proposal.Version = 1
	if err := Apply(&bundle, proposal, Verification{Method: "test", Evidence: "go test ./internal/knowledge"}); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadProposals(t *testing.T) {
	t.Parallel()

	bundle := Bundle{Concepts: map[string]Concept{
		"deploy-freeze": {ID: "deploy-freeze", Title: "Deploy Freeze", Version: 2, Status: "active", Body: "old", Path: "deploy-freeze.md"},
	}}
	mk := func(mut func(*Proposal)) Proposal {
		p := validProposal()
		mut(&p)
		return p
	}
	proposals := []struct {
		name     string
		proposal Proposal
		verify   Verification
	}{
		{name: "no sources", proposal: mk(func(p *Proposal) { p.Sources = nil }), verify: humanVerification()},
		{name: "no reason", proposal: mk(func(p *Proposal) { p.Reason = "" }), verify: humanVerification()},
		{name: "skipped version", proposal: mk(func(p *Proposal) { p.Version = 4 }), verify: humanVerification()},
		{name: "same version", proposal: mk(func(p *Proposal) { p.Version = 2 }), verify: humanVerification()},
		{name: "bad method", proposal: validProposal(), verify: Verification{Method: "vibes", Evidence: "x"}},
		{name: "no evidence", proposal: validProposal(), verify: Verification{Method: "gate"}},
		{name: "oversized body", proposal: mk(func(p *Proposal) { p.Body = strings.Repeat("b", 100001) }), verify: humanVerification()},
	}
	for _, tc := range proposals {
		t.Run(tc.name, func(t *testing.T) {
			fresh := Bundle{Concepts: map[string]Concept{}}
			for id, c := range bundle.Concepts {
				fresh.Concepts[id] = c
			}
			if err := Apply(&fresh, tc.proposal, tc.verify); err == nil {
				t.Fatalf("expected rejection for %s, got none", tc.name)
			}
		})
	}
}

func TestDeprecationNeedsHuman(t *testing.T) {
	t.Parallel()

	bundle := Bundle{Concepts: map[string]Concept{
		"deploy-freeze": {ID: "deploy-freeze", Title: "Deploy Freeze", Version: 2, Status: "active", Body: "old", Path: "deploy-freeze.md"},
	}}
	proposal := validProposal()
	proposal.Status = "deprecated"
	if err := Apply(&bundle, proposal, Verification{Method: "gate", Evidence: "checks green"}); err == nil {
		t.Fatal("expected human requirement, got none")
	}
	if err := Apply(&bundle, proposal, humanVerification()); err != nil {
		t.Fatal(err)
	}
}
