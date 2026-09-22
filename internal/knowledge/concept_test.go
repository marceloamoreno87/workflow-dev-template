package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validConcept = `---
id: deploy-freeze
title: Deploy Freeze
version: 2
status: active
tags: [delivery, policy]
---

Freeze production deploys on Fridays.
`

func TestParseConcept(t *testing.T) {
	t.Parallel()

	c, err := ParseConcept("deploy-freeze.md", []byte(validConcept))
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "deploy-freeze" || c.Version != 2 || c.Status != "active" || len(c.Tags) != 2 {
		t.Fatalf("unexpected concept: %#v", c)
	}
	if !strings.Contains(c.Body, "Fridays") {
		t.Fatalf("body lost: %q", c.Body)
	}
}

func TestRejectBadConcepts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  string
	}{
		{name: "no frontmatter", doc: "# Just markdown\n"},
		{name: "unclosed fence", doc: "---\nid: x\n"},
		{name: "bad yaml", doc: "---\nid: [x\n---\nbody\n"},
		{name: "bad id", doc: "---\nid: Bad_ID!\ntitle: t\nversion: 1\nstatus: active\n---\nbody\n"},
		{name: "zero version", doc: "---\nid: x\ntitle: t\nversion: 0\nstatus: active\n---\nbody\n"},
		{name: "bad status", doc: "---\nid: x\ntitle: t\nversion: 1\nstatus: draft\n---\nbody\n"},
		{name: "empty title", doc: "---\nid: x\nversion: 1\nstatus: active\n---\nbody\n"},
		{name: "empty body", doc: "---\nid: x\ntitle: t\nversion: 1\nstatus: active\n---\n"},
		{name: "body too long", doc: "---\nid: x\ntitle: t\nversion: 1\nstatus: active\n---\n" + strings.Repeat("b", 100001)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseConcept("x.md", []byte(tc.doc)); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestLoadBundle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deploy-freeze.md"), []byte(validConcept), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte(validConcept), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle, err := LoadBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Concepts) != 1 {
		t.Fatalf("expected 1 concept, got %d", len(bundle.Concepts))
	}
	if _, err := LoadBundle(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected missing dir rejection, got none")
	}
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "broken.md"), []byte("# no frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBundle(broken); err == nil {
		t.Fatal("expected broken bundle rejection, got none")
	}
}
