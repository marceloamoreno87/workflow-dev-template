# Knowledge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store Project knowledge as versioned OKF Concepts on disk, retrieve them through a deterministic in-memory full-text index with snippets, and evolve the bundle only through sourced proposals with impact-appropriate verification.

**Architecture:** `knowledge` owns Concept parsing, bundle loading, inverted-index search, proposal validation, and verified application; it reads the filesystem but never touches the network. It may import the standard library plus `gopkg.in/yaml.v3` (frontmatter shares the YAML shape of manifests and registries per contracts). All other modules stay untouched with no new imports. SQLite FTS5 migration, embeddings, cross-Project search, and daemon-side proposal sync belong to later increments and are explicitly out of scope: the in-memory index is the v1 spike with identical ranking semantics re-implementable over FTS later.

**Tech Stack:** Go 1.27.1 standard library plus `gopkg.in/yaml.v3`, fixture bundles in `t.TempDir()`, table-driven tests

**Spec:** `docs/architecture.md` (Project knowledge as OKF bundles under `.knowledge/`; Module Knowledge; no utils/common/manager), `docs/adr/0003-store-project-knowledge-as-okf.md`, `CONTEXT.md` (Knowledge Base, Knowledge Bundle, Knowledge Concept, Knowledge Proposal, Improvement Proposal), `docs/contracts.md` (structured versioned artifacts)

## Global Constraints

- Use Go 1.27.1.
- Standard library plus `gopkg.in/yaml.v3` only; do not add dependencies.
- `internal/knowledge` imports nothing else outside those.
- Use the canonical terms from `CONTEXT.md` (Knowledge Bundle, Knowledge Concept, Knowledge Proposal, Improvement Proposal); never write document, page, or memory for a Concept.
- Concept files are Markdown with `---` YAML frontmatter (`id`, `title`, `version`, `status`, `tags`) followed by a body; missing or malformed frontmatter fails the file closed with its path, never silently.
- Concept ids match `[a-z0-9][a-z0-9._-]{0,127}`; versions start at 1 and increment by exactly 1; status is `active` or `deprecated`; bodies are 1..100000 runes; bundles hold at most 1024 Concepts.
- Search is deterministic: same bundle plus query always yields the same order (score descending, id ascending). Tokenization lowercases alphanumeric runs of length ≥ 2; title hits weigh 3, exact tag hits weigh 5, body hits weigh 1.
- Every Knowledge Proposal carries at least one source and a reason; existing Concepts change only by exact +1 versions; deprecation requires human verification.
- Errors name file paths but never echo file bodies.
- Do not add generic repository, provider, manager, service, utils, or common packages; new code lives in `internal/knowledge` only.

---

### Task 1: Parse Concepts and load Bundles

**Files:**
- Create: `internal/knowledge/concept.go`
- Test: `internal/knowledge/concept_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `Concept struct`, `ParseConcept(path string, data []byte) (Concept, error)`, `Bundle struct`, `LoadBundle(dir string) (Bundle, error)`, sentinel `ErrKnowledge`.

- [ ] **Step 1: Write the failing Concept test**

```go
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
```

- [ ] **Step 2: Run the test and verify the package does not compile**

Run: `go test ./internal/knowledge -run 'TestParseConcept|TestRejectBadConcepts|TestLoadBundle' -count=1`

Expected: FAIL because `Concept`, `ParseConcept`, `Bundle`, `LoadBundle`, and `ErrKnowledge` are undefined.

- [ ] **Step 3: Implement Concept parsing and bundle loading**

```go
// internal/knowledge/concept.go
package knowledge

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var ErrKnowledge = errors.New("invalid knowledge concept")

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

const maxBodyRunes = 100000

const maxBundleFiles = 1024

type Concept struct {
	ID     string
	Title  string
	Version int
	Status string
	Tags   []string
	Body   string
	Path   string
}

type Bundle struct {
	Concepts map[string]Concept
}

func ParseConcept(path string, data []byte) (Concept, error) {
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return Concept{}, fmt.Errorf("%w: %s missing frontmatter", ErrKnowledge, path)
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return Concept{}, fmt.Errorf("%w: %s unclosed frontmatter", ErrKnowledge, path)
	}
	var front struct {
		ID      string   `yaml:"id"`
		Title   string   `yaml:"title"`
		Version int      `yaml:"version"`
		Status  string   `yaml:"status"`
		Tags    []string `yaml:"tags"`
	}
	if err := yaml.Unmarshal([]byte(rest[:end]), &front); err != nil {
		return Concept{}, fmt.Errorf("%w: %s bad frontmatter", ErrKnowledge, path)
	}
	body := strings.TrimSpace(rest[end+len("\n---\n"):])
	if !idPattern.MatchString(front.ID) {
		return Concept{}, fmt.Errorf("%w: %s bad id", ErrKnowledge, path)
	}
	if strings.TrimSpace(front.Title) == "" || utf8.RuneCountInString(front.Title) > 200 {
		return Concept{}, fmt.Errorf("%w: %s bad title", ErrKnowledge, path)
	}
	if front.Version < 1 {
		return Concept{}, fmt.Errorf("%w: %s bad version", ErrKnowledge, path)
	}
	if front.Status != "active" && front.Status != "deprecated" {
		return Concept{}, fmt.Errorf("%w: %s bad status", ErrKnowledge, path)
	}
	if n := utf8.RuneCountInString(body); n == 0 || n > maxBodyRunes {
		return Concept{}, fmt.Errorf("%w: %s body length %d", ErrKnowledge, path, n)
	}
	return Concept{ID: front.ID, Title: strings.TrimSpace(front.Title), Version: front.Version, Status: front.Status, Tags: front.Tags, Body: body, Path: path}, nil
}

func LoadBundle(dir string) (Bundle, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Bundle{}, fmt.Errorf("%w: unreadable bundle %s", ErrKnowledge, dir)
	}
	bundle := Bundle{Concepts: map[string]Concept{}}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, ".") {
			continue
		}
		if len(bundle.Concepts) >= maxBundleFiles {
			return Bundle{}, fmt.Errorf("%w: bundle too large", ErrKnowledge)
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return Bundle{}, fmt.Errorf("%w: unreadable %s", ErrKnowledge, name)
		}
		concept, err := ParseConcept(name, raw)
		if err != nil {
			return Bundle{}, err
		}
		if _, dup := bundle.Concepts[concept.ID]; dup {
			return Bundle{}, fmt.Errorf("%w: duplicate concept %q", ErrKnowledge, concept.ID)
		}
		bundle.Concepts[concept.ID] = concept
		_ = fs.ValidPath
	}
	return bundle, nil
}
```

NOTE: the `_ = fs.ValidPath` line is a leftover to drop when writing the file (no `io/fs` import needed). Load only the top level of the bundle directory: nested knowledge trees belong to a later increment.

- [ ] **Step 4: Run the package test**

Run: `go test ./internal/knowledge -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/knowledge/concept.go internal/knowledge/concept_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/knowledge/concept.go internal/knowledge/concept_test.go
git commit -m "feat(knowledge): parse concepts and load bundles"
```

### Task 2: Index and search with snippets

**Files:**
- Create: `internal/knowledge/search.go`
- Test: `internal/knowledge/search_test.go`

**Interfaces:**
- Consumes: `Bundle` from Task 1.
- Produces: `Result struct`, `Search(bundle Bundle, query string, limit int) ([]Result, error)` with title×3, tag×5, body×1 scoring and one snippet per hit.

- [ ] **Step 1: Write the failing search test**

```go
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
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/knowledge -run 'TestSearch' -count=1`

Expected: FAIL because `Search` and `Result` are undefined.

- [ ] **Step 3: Implement the inverted index search**

```go
// internal/knowledge/search.go
package knowledge

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type Result struct {
	ID      string
	Score   int
	Snippet string
	Terms   []string
}

func tokenize(s string) []string {
	var out []string
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		if b.Len() >= 2 {
			out = append(out, b.String())
		}
		b.Reset()
	}
	if b.Len() >= 2 {
		out = append(out, b.String())
	}
	return out
}

func countOccurrences(tokens []string, term string) int {
	n := 0
	for _, t := range tokens {
		if t == term {
			n++
		}
	}
	return n
}

func snippetFor(body string, terms []string) string {
	lower := strings.ToLower(body)
	best, bestIdx := "", -1
	for _, term := range terms {
		if idx := strings.Index(lower, term); idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
			bestIdx = idx
			best = term
		}
	}
	if bestIdx < 0 {
		runes := []rune(body)
		if len(runes) > 120 {
			return string(runes[:120]) + "…"
		}
		return body
	}
	runes := []rune(body)
	lowerRunes := []rune(lower)
	termRunes := []rune(best)
	start := 0
	for i, r := range lowerRunes {
		_ = r
		if i+len(termRunes) <= len(lowerRunes) && string(lowerRunes[i:i+len(termRunes)]) == best {
			start = i
			break
		}
	}
	from := start - 60
	if from < 0 {
		from = 0
	}
	to := start + len(termRunes) + 60
	if to > len(runes) {
		to = len(runes)
	}
	window := string(runes[from:to])
	if from > 0 {
		window = "…" + window
	}
	if to < len(runes) {
		window += "…"
	}
	return window
}

func Search(bundle Bundle, query string, limit int) ([]Result, error) {
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil, fmt.Errorf("%w: empty query", ErrKnowledge)
	}
	if limit <= 0 || limit > 50 {
		return nil, fmt.Errorf("%w: limit outside 1..50", ErrKnowledge)
	}
	var out []Result
	for id, concept := range bundle.Concepts {
		score := 0
		matched := map[string]bool{}
		titleTokens := tokenize(concept.Title)
		bodyTokens := tokenize(concept.Title + " " + concept.Body)
		for _, term := range terms {
			titleHits := countOccurrences(titleTokens, term)
			bodyHits := countOccurrences(bodyTokens, term) - titleHits
			if bodyHits < 0 {
				bodyHits = 0
			}
			tagHit := false
			for _, tag := range concept.Tags {
				if strings.ToLower(tag) == term {
					tagHit = true
					break
				}
			}
			if titleHits == 0 && bodyHits == 0 && !tagHit {
				continue
			}
			matched[term] = true
			score += 3*titleHits + bodyHits
			if tagHit {
				score += 5
			}
		}
		if score == 0 {
			continue
		}
		termList := make([]string, 0, len(matched))
		for term := range matched {
			termList = append(termList, term)
		}
		sort.Strings(termList)
		out = append(out, Result{ID: id, Score: score, Snippet: snippetFor(concept.Body, termList), Terms: termList})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
```

NOTE: the `for i, r := range lowerRunes` loop with `_ = r` is clumsy; when writing the file, simplify the window-start computation to a rune-aware index search without the dead loop (e.g. locate via strings.Index on the lowercased string, then convert the byte offset to a rune offset with `len([]rune(lower[:idx]))`).

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/knowledge -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/knowledge/search.go internal/knowledge/search_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/knowledge/search.go internal/knowledge/search_test.go
git commit -m "feat(knowledge): index and search with snippets"
```

### Task 3: Propose changes with verification

**Files:**
- Create: `internal/knowledge/proposal.go`
- Test: `internal/knowledge/proposal_test.go`

**Interfaces:**
- Consumes: `Concept`/`Bundle` from Task 1.
- Produces: `Proposal struct`, `Verification struct`, `(Proposal).Validate() error`, `Apply(bundle *Bundle, p Proposal, v Verification) error`.

- [ ] **Step 1: Write the failing proposal test**

```go
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
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/knowledge -run 'TestApply|TestRejectBadProposals|TestDeprecation' -count=1`

Expected: FAIL because `Proposal`, `Verification`, and `Apply` are undefined.

- [ ] **Step 3: Implement proposals with verification**

```go
// internal/knowledge/proposal.go
package knowledge

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Proposal struct {
	ConceptID string
	Title     string
	Body      string
	Sources   []string
	Reason    string
	Version   int
	Status    string
}

type Verification struct {
	Method   string
	Evidence string
}

func (p Proposal) Validate() error {
	if !idPattern.MatchString(p.ConceptID) {
		return fmt.Errorf("%w: concept id", ErrKnowledge)
	}
	if strings.TrimSpace(p.Title) == "" || utf8.RuneCountInString(p.Title) > 200 {
		return fmt.Errorf("%w: title", ErrKnowledge)
	}
	if n := utf8.RuneCountInString(p.Body); n == 0 || n > maxBodyRunes {
		return fmt.Errorf("%w: body length %d", ErrKnowledge, n)
	}
	if len(p.Sources) == 0 {
		return fmt.Errorf("%w: at least one source", ErrKnowledge)
	}
	for _, s := range p.Sources {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%w: empty source", ErrKnowledge)
		}
	}
	if strings.TrimSpace(p.Reason) == "" || utf8.RuneCountInString(p.Reason) > 2000 {
		return fmt.Errorf("%w: reason", ErrKnowledge)
	}
	if p.Version < 1 {
		return fmt.Errorf("%w: version", ErrKnowledge)
	}
	if p.Status != "" && p.Status != "active" && p.Status != "deprecated" {
		return fmt.Errorf("%w: status", ErrKnowledge)
	}
	return nil
}

func (v Verification) Validate() error {
	switch v.Method {
	case "human", "gate", "test":
	default:
		return fmt.Errorf("%w: verification method %q", ErrKnowledge, v.Method)
	}
	if strings.TrimSpace(v.Evidence) == "" || utf8.RuneCountInString(v.Evidence) > 2000 {
		return fmt.Errorf("%w: verification evidence", ErrKnowledge)
	}
	return nil
}

func Apply(bundle *Bundle, p Proposal, v Verification) error {
	if bundle == nil {
		return fmt.Errorf("%w: nil bundle", ErrKnowledge)
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if err := v.Validate(); err != nil {
		return err
	}
	current, exists := bundle.Concepts[p.ConceptID]
	if !exists {
		if p.Version != 1 {
			return fmt.Errorf("%w: new concepts start at version 1", ErrKnowledge)
		}
		status := p.Status
		if status == "" {
			status = "active"
		}
		bundle.Concepts[p.ConceptID] = Concept{ID: p.ConceptID, Title: strings.TrimSpace(p.Title), Version: 1, Status: status, Body: strings.TrimSpace(p.Body), Path: p.ConceptID + ".md"}
		return nil
	}
	if p.Version != current.Version+1 {
		return fmt.Errorf("%w: version must advance by exactly one", ErrKnowledge)
	}
	status := current.Status
	if p.Status != "" {
		status = p.Status
	}
	if status == "deprecated" && current.Status != "deprecated" && v.Method != "human" {
		return fmt.Errorf("%w: deprecation needs human verification", ErrKnowledge)
	}
	tags := current.Tags
	bundle.Concepts[p.ConceptID] = Concept{ID: p.ConceptID, Title: strings.TrimSpace(p.Title), Version: p.Version, Status: status, Tags: tags, Body: strings.TrimSpace(p.Body), Path: current.Path}
	return nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/knowledge -count=1`

Expected: PASS.

- [ ] **Step 5: Format and commit**

Run: `gofmt -w internal/knowledge/proposal.go internal/knowledge/proposal_test.go && go test ./...`

Expected: PASS.

```bash
git add internal/knowledge/proposal.go internal/knowledge/proposal_test.go
git commit -m "feat(knowledge): propose changes with verification"
```

### Task 4: Prove the bundle lifecycle end-to-end

**Files:**
- Create: `internal/knowledge/flow_test.go`
- Modify: `docs/implementation-plan.md`

**Interfaces:**
- Consumes: complete bundle lifecycle from Tasks 1-3.
- Produces: executable evidence that a disk bundle loads, answers search, accepts a verified proposal, and answers the new terms — while unverified or unsourced proposals never land.

- [ ] **Step 1: Add the flow test**

```go
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
```

NOTE: `humanVerification()` is defined in `proposal_test.go` (same package) — reuse it here rather than redefining.

- [ ] **Step 2: Run the flow test**

Run: `go test ./internal/knowledge -run 'TestBundleLifecycle' -count=1`

Expected: PASS.

- [ ] **Step 3: Run the full verification set**

Run: `go test -race ./...`

Expected: PASS with zero failures and zero race reports.

Run: `go vet ./...`

Expected: exit 0 with no findings.

Run: `go build ./...`

Expected: clean build of `cmd/harness`.

- [ ] **Step 4: Mark Increment 14 verified and commit**

In `docs/implementation-plan.md`, add Increment 14 to the plan index and a verification checklist below Increment 13. Do not mark it complete until the commands above pass on master.

```bash
git add internal/knowledge/flow_test.go docs/implementation-plan.md
git commit -m "test(knowledge): verify bundle lifecycle"
```
