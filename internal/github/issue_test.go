package github

import (
	"strings"
	"testing"
)

const validIssue = `{"repository": "owner/repo", "number": 123, "title": "Fix login", "body": "Steps to reproduce", "state": "open", "author": "octocat"}`

func TestParseValidIssue(t *testing.T) {
	t.Parallel()

	issue, err := ParseIssue([]byte(validIssue))
	if err != nil {
		t.Fatal(err)
	}
	if string(issue.ID) != "owner/repo#123" {
		t.Fatalf("unexpected id: %q", issue.ID)
	}
	if issue.Title != "Fix login" || issue.State != IssueOpen || issue.Number != 123 {
		t.Fatalf("unexpected issue: %#v", issue)
	}
}

func TestRejectBadIssues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  string
	}{
		{name: "not json", doc: `{"repository":`},
		{name: "missing repository", doc: `{"number": 1, "title": "x", "state": "open", "author": "a"}`},
		{name: "bad repository", doc: `{"repository": "owner", "number": 1, "title": "x", "state": "open", "author": "a"}`},
		{name: "zero number", doc: `{"repository": "owner/repo", "number": 0, "title": "x", "state": "open", "author": "a"}`},
		{name: "empty title", doc: `{"repository": "owner/repo", "number": 1, "title": "  ", "state": "open", "author": "a"}`},
		{name: "title too long", doc: `{"repository": "owner/repo", "number": 1, "title": "` + strings.Repeat("x", 301) + `", "state": "open", "author": "a"}`},
		{name: "body too long", doc: `{"repository": "owner/repo", "number": 1, "title": "x", "body": "` + strings.Repeat("y", 20001) + `", "state": "open", "author": "a"}`},
		{name: "bad state", doc: `{"repository": "owner/repo", "number": 1, "title": "x", "state": "merged", "author": "a"}`},
		{name: "missing author", doc: `{"repository": "owner/repo", "number": 1, "title": "x", "state": "open"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseIssue([]byte(tc.doc)); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}

func TestIssueSanitizesControlCharacters(t *testing.T) {
	t.Parallel()

	issue, err := ParseIssue([]byte(`{"repository": "owner/repo", "number": 7, "title": "a\u0007b", "body": "x\u0001y", "state": "open", "author": "octocat"}`))
	if err != nil {
		t.Fatal(err)
	}
	if issue.Title != "ab" || issue.Body != "xy" {
		t.Fatalf("controls not stripped: %#v", issue)
	}
}
