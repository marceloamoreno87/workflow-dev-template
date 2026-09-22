package knowledge

import (
	"strings"
	"testing"
)

func fixtureBundle() Bundle {
	must := func(path, doc string) Concept {
		c, err := ParseConcept(path, []byte(doc))
		if err != nil {
			panic(err)
		}
		return c
	}
	return Bundle{Concepts: map[string]Concept{
		"deploy-freeze": must("deploy-freeze.md", validConcept),
		"flag-hygiene": must("flag-hygiene.md", `---
id: flag-hygiene
title: Flag Removal Hygiene
version: 1
status: active
tags: [flags]
---

Remove stale feature flags within fourteen days of rollout.
`),
		"retired-note": must("retired-note.md", `---
id: retired-note
title: Old Runbook
version: 1
status: deprecated
tags: [ops]
---

This runbook no longer applies to Fridays.
`),
	}}
}

func TestSearchRanksTitleOverBody(t *testing.T) {
	t.Parallel()

	results, err := Search(fixtureBundle(), "flag removal", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 || results[0].ID != "flag-hygiene" {
		t.Fatalf("title hit should rank first: %#v", results)
	}
}

func TestSearchFindsBodyTerms(t *testing.T) {
	t.Parallel()

	results, err := Search(fixtureBundle(), "Fridays", 10)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, r := range results {
		ids[r.ID] = true
	}
	if !ids["deploy-freeze"] || !ids["retired-note"] {
		t.Fatalf("body terms missing: %#v", results)
	}
	for _, r := range results {
		if r.Snippet == "" || !strings.Contains(strings.ToLower(r.Snippet), "fridays") {
			t.Fatalf("snippet missing term: %#v", r)
		}
	}
}

func TestSearchIsDeterministic(t *testing.T) {
	t.Parallel()

	first, err := Search(fixtureBundle(), "flag", 10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Search(fixtureBundle(), "flag", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) {
		t.Fatal("unstable result count")
	}
	for i := range first {
		if first[i].ID != second[i].ID || first[i].Score != second[i].Score {
			t.Fatal("unstable ordering")
		}
	}
}

func TestRejectBadQueries(t *testing.T) {
	t.Parallel()

	for _, q := range []string{"", "   ", "a"} {
		if _, err := Search(fixtureBundle(), q, 10); err == nil {
			t.Fatalf("expected rejection for %q, got none", q)
		}
	}
}
