package registry

import (
	"testing"
)

const validRegistry = `schemaVersion: harness.registry/v1
projects:
  - id: example
    path: projects/example
    github:
      repository: owner/repo
      clientProject: PVT_client
      portfolioProject: PVT_private
    actors:
      operator: actor/operator
      clients: [actor/client-a]
`

func TestParseValidRegistry(t *testing.T) {
	t.Parallel()

	r, err := ParseRegistry([]byte(validRegistry))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Projects) != 1 || r.Projects[0].ID != "example" {
		t.Fatalf("unexpected registry: %#v", r.Projects)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadRegistries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  string
	}{
		{name: "missing schema", doc: "projects: []\n"},
		{name: "wrong schema", doc: "schemaVersion: harness/v9\nprojects: []\n"},
		{name: "absolute path", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: /etc/passwd\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n"},
		{name: "escape", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: projects/../secret\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n"},
		{name: "outside projects", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: other/a\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n"},
		{name: "duplicate id", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: projects/a\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n  - id: a\n    path: projects/b\n    github:\n      repository: o/r\n    actors:\n      operator: actor/operator\n"},
		{name: "missing repository", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: projects/a\n    github: {}\n    actors:\n      operator: actor/operator\n"},
		{name: "missing operator", doc: "schemaVersion: harness.registry/v1\nprojects:\n  - id: a\n    path: projects/a\n    github:\n      repository: o/r\n    actors: {}\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ParseRegistry([]byte(tc.doc))
			if err != nil {
				return
			}
			if err := r.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
