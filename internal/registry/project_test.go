package registry

import (
	"testing"
)

const validManifest = `schemaVersion: harness/v1
project:
  id: example
  repository: owner/repo
  defaultBranch: main
stack:
  adapter: go-service
quality:
  profile: standard
data:
  classification: internal
knowledge:
  root: .knowledge
specs:
  root: .specs
delivery:
  coolifyResource: resource-id
  productionUrl: https://example.com
flags:
  provider: go-feature-flag
  declarationRoot: .flags
budgets:
  maxDuration: 2h
  maxCostUSD: 10
  maxFixCycles: 3
`

func TestParseValidManifest(t *testing.T) {
	t.Parallel()

	m, err := ParseProjectManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	if m.Project.ID != "example" || m.Project.Repository != "owner/repo" {
		t.Fatalf("unexpected manifest: %#v", m.Project)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := RejectSecrets([]byte(validManifest)); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadManifests(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		doc  string
	}{
		{name: "missing schema", doc: "project:\n  id: x\n"},
		{name: "wrong schema", doc: "schemaVersion: harness/v9\nproject:\n  id: x\n  repository: o/r\n"},
		{name: "bad profile", doc: "schemaVersion: harness/v1\nproject:\n  id: x\n  repository: o/r\nstack:\n  adapter: go-service\nquality:\n  profile: turbo\ndata:\n  classification: internal\n"},
		{name: "bad classification", doc: "schemaVersion: harness/v1\nproject:\n  id: x\n  repository: o/r\nstack:\n  adapter: go-service\nquality:\n  profile: standard\ndata:\n  classification: topsecret\n"},
		{name: "secret key", doc: "schemaVersion: harness/v1\nproject:\n  id: x\n  repository: o/r\ndelivery:\n  apiToken: abc\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := ParseProjectManifest([]byte(tc.doc))
			if err == nil {
				if verr := m.Validate(); verr == nil {
					if serr := RejectSecrets([]byte(tc.doc)); serr == nil {
						t.Fatal("expected rejection, got none")
					}
				}
			}
		})
	}
}
