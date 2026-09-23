// internal/githublive/pullrequest.go
package githublive

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var branchPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,256}$`)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type NewPR struct {
	Base  string
	Head  string
	Title string
	Body  string
}

func (p NewPR) Validate() error {
	if p.Base == "" || !branchPattern.MatchString(p.Base) || strings.Contains(p.Base, "..") {
		return fmt.Errorf("%w: base branch", ErrGitHub)
	}
	if p.Head == "" || !branchPattern.MatchString(p.Head) || strings.Contains(p.Head, "..") {
		return fmt.Errorf("%w: head branch", ErrGitHub)
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Title)); n == 0 || n > 256 {
		return fmt.Errorf("%w: title length %d", ErrGitHub, n)
	}
	if utf8.RuneCountInString(p.Body) > 65536 {
		return fmt.Errorf("%w: body too long", ErrGitHub)
	}
	return nil
}

type PR struct {
	Number int
	Head   string
}

type Merge struct {
	Merged bool
	SHA    string
}

func (c Client) OpenPR(ctx context.Context, owner, repo string, pr NewPR) (PR, Call, error) {
	if err := validOwnerRepo(owner, repo); err != nil {
		return PR{}, Call{}, err
	}
	if err := pr.Validate(); err != nil {
		return PR{}, Call{}, err
	}
	body, call, err := c.post(ctx, "/repos/"+owner+"/"+repo+"/pulls", map[string]string{
		"base": pr.Base, "head": pr.Head, "title": pr.Title, "body": pr.Body,
	})
	if err != nil {
		return PR{}, call, err
	}
	var doc struct {
		Number int `json:"number"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return PR{}, call, fmt.Errorf("%w: decode pull request", ErrGitHub)
	}
	if doc.Number <= 0 || !shaPattern.MatchString(doc.Head.SHA) {
		return PR{}, call, fmt.Errorf("%w: malformed pull request", ErrGitHub)
	}
	return PR{Number: doc.Number, Head: doc.Head.SHA}, call, nil
}

func (c Client) MergePR(ctx context.Context, owner, repo string, number int, method string) (Merge, Call, error) {
	if err := validOwnerRepo(owner, repo); err != nil {
		return Merge{}, Call{}, err
	}
	if number <= 0 {
		return Merge{}, Call{}, fmt.Errorf("%w: pull request number", ErrGitHub)
	}
	if method != "merge" {
		return Merge{}, Call{}, fmt.Errorf("%w: only merge preserves commits", ErrGitHub)
	}
	body, call, err := c.put(ctx, fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, number), map[string]string{
		"merge_method": "merge",
	})
	if err != nil {
		return Merge{}, call, err
	}
	var doc struct {
		Merged bool   `json:"merged"`
		SHA    string `json:"sha"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return Merge{}, call, fmt.Errorf("%w: decode merge", ErrGitHub)
	}
	if !doc.Merged {
		return Merge{}, call, fmt.Errorf("%w: pull request not merged", ErrGitHub)
	}
	if !shaPattern.MatchString(doc.SHA) {
		return Merge{}, call, fmt.Errorf("%w: malformed merge sha", ErrGitHub)
	}
	return Merge{Merged: true, SHA: doc.SHA}, call, nil
}

func validOwnerRepo(owner, repo string) error {
	if !ownerPattern.MatchString(owner) || !ownerPattern.MatchString(repo) {
		return fmt.Errorf("%w: owner/repo", ErrGitHub)
	}
	return nil
}
