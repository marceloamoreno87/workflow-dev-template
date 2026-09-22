package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundleLifecycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	doc := `---
id: deploy-freeze
title: Deploy Freeze
version: 1
status: active
tags: [delivery]
---

Freeze production deploys on Fridays.
`
	if err := os.WriteFile(filepath.Join(dir, "deploy-freeze.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle, err := LoadBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := Search(bundle, "Fridays", 10)
	if err != nil || len(before) != 1 {
		t.Fatalf("expected one hit: %#v %v", before, err)
	}
	proposal := Proposal{
		ConceptID: "deploy-freeze",
		Title:     "Deploy Freeze",
		Body:      "Freeze production deploys on Fridays and holidays.",
		Sources:   []string{"owner/repo#123"},
		Reason:    "holidays added by operator review",
		Version:   2,
	}
	if err := Apply(&bundle, proposal, Verification{Method: "vibes", Evidence: "x"}); err == nil {
		t.Fatal("unverified proposal landed")
	}
	unsourced := proposal
	unsourced.Sources = nil
	if err := Apply(&bundle, unsourced, humanVerification()); err == nil {
		t.Fatal("unsourced proposal landed")
	}
	if err := Apply(&bundle, proposal, humanVerification()); err != nil {
		t.Fatal(err)
	}
	after, err := Search(bundle, "holidays", 10)
	if err != nil || len(after) != 1 || after[0].ID != "deploy-freeze" {
		t.Fatalf("new terms not searchable: %#v %v", after, err)
	}
}
