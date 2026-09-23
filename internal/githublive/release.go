// internal/githublive/release.go
package githublive

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
)

var tagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type Release struct {
	ID  int
	Tag string
}

type Commit struct {
	SHA    string
	Author string
}

func (c Client) CreateRelease(ctx context.Context, owner, repo, tag, commit, name, body string) (Release, Call, error) {
	if err := validOwnerRepo(owner, repo); err != nil {
		return Release{}, Call{}, err
	}
	if !tagPattern.MatchString(tag) {
		return Release{}, Call{}, fmt.Errorf("%w: tag %q", ErrGitHub, tag)
	}
	if !shaPattern.MatchString(commit) {
		return Release{}, Call{}, fmt.Errorf("%w: commit", ErrGitHub)
	}
	raw, call, err := c.post(ctx, "/repos/"+owner+"/"+repo+"/releases", map[string]string{
		"tag_name": tag, "target_commitish": commit, "name": name, "body": body,
	})
	if err != nil {
		return Release{}, call, err
	}
	var doc struct {
		ID      int    `json:"id"`
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Release{}, call, fmt.Errorf("%w: decode release", ErrGitHub)
	}
	if doc.ID <= 0 || doc.TagName != tag {
		return Release{}, call, fmt.Errorf("%w: malformed release", ErrGitHub)
	}
	return Release{ID: doc.ID, Tag: doc.TagName}, call, nil
}

func (c Client) ListCommits(ctx context.Context, owner, repo string, number int) ([]Commit, Call, error) {
	if err := validOwnerRepo(owner, repo); err != nil {
		return nil, Call{}, err
	}
	if number <= 0 {
		return nil, Call{}, fmt.Errorf("%w: pull request number", ErrGitHub)
	}
	raw, call, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/pulls/%d/commits", owner, repo, number), "")
	if err != nil {
		return nil, call, err
	}
	var docs []struct {
		SHA    string `json:"sha"`
		Author *struct {
			Login string `json:"login"`
		} `json:"author"`
		Commit struct {
			Author struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(raw, &docs); err != nil {
		return nil, call, fmt.Errorf("%w: decode commits", ErrGitHub)
	}
	commits := make([]Commit, 0, len(docs))
	for _, doc := range docs {
		if !shaPattern.MatchString(doc.SHA) {
			return nil, call, fmt.Errorf("%w: malformed commit sha", ErrGitHub)
		}
		author := doc.Commit.Author.Name
		if doc.Author != nil && doc.Author.Login != "" {
			author = doc.Author.Login
		}
		commits = append(commits, Commit{SHA: doc.SHA, Author: author})
	}
	return commits, call, nil
}

// HasForeignCommits reports whether any commit falls outside the harness author,
// in which case reorganization is forbidden (delivery policy).
func HasForeignCommits(commits []Commit, harnessAuthor string) bool {
	for _, c := range commits {
		if c.Author != harnessAuthor {
			return true
		}
	}
	return false
}
