// internal/delivery/pullrequest.go
package delivery

import (
	"errors"
	"fmt"
	"regexp"
)

var ErrDelivery = errors.New("invalid delivery transition")

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type Commit struct {
	SHA        string
	Author     string
	ThirdParty bool
}

type PullRequest struct {
	Base               string
	Head               string
	Commits            []Commit
	Created            bool
	ApprovedByOperator bool
	ChecksPassed       bool
	MergeMethod        string
}

func validSHA(sha string) bool {
	return shaPattern.MatchString(sha)
}

func checkRange(commits []Commit) error {
	if len(commits) == 0 {
		return fmt.Errorf("%w: empty commit range", ErrDelivery)
	}
	for _, c := range commits {
		if !validSHA(c.SHA) {
			return fmt.Errorf("%w: malformed commit sha", ErrDelivery)
		}
		if c.Author == "" {
			return fmt.Errorf("%w: commit without author", ErrDelivery)
		}
	}
	return nil
}

func CanReorganize(commits []Commit, prCreated bool) error {
	if err := checkRange(commits); err != nil {
		return err
	}
	if prCreated {
		return fmt.Errorf("%w: range frozen after PR creation", ErrDelivery)
	}
	for _, c := range commits {
		if c.ThirdParty {
			return fmt.Errorf("%w: third-party commits are never rewritten", ErrDelivery)
		}
	}
	return nil
}

func CanMerge(pr PullRequest) error {
	if !pr.Created {
		return fmt.Errorf("%w: pull request not created", ErrDelivery)
	}
	if pr.Base == "" {
		return fmt.Errorf("%w: base branch required", ErrDelivery)
	}
	if !validSHA(pr.Head) {
		return fmt.Errorf("%w: head must be a commit sha", ErrDelivery)
	}
	if err := checkRange(pr.Commits); err != nil {
		return err
	}
	if !pr.ApprovedByOperator {
		return fmt.Errorf("%w: operator approval required", ErrDelivery)
	}
	if !pr.ChecksPassed {
		return fmt.Errorf("%w: required checks must pass", ErrDelivery)
	}
	if pr.MergeMethod != "merge" {
		return fmt.Errorf("%w: only merge preserves commits", ErrDelivery)
	}
	return nil
}
