// internal/github/issue.go
package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

var ErrIssue = errors.New("invalid github issue")

type IssueState string

const (
	IssueOpen   IssueState = "open"
	IssueClosed IssueState = "closed"
)

type IntakeIssue struct {
	ID         workflow.WorkItemID
	Repository string
	Number     int
	Title      string
	Body       string
	State      IssueState
	Author     string
}

func ParseIssue(data []byte) (IntakeIssue, error) {
	var raw struct {
		Repository string `json:"repository"`
		Number     int    `json:"number"`
		Title      string `json:"title"`
		Body       string `json:"body"`
		State      string `json:"state"`
		Author     string `json:"author"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return IntakeIssue{}, err
	}
	if !validRepository(raw.Repository) {
		return IntakeIssue{}, fmt.Errorf("%w: repository %q", ErrIssue, raw.Repository)
	}
	if raw.Number <= 0 {
		return IntakeIssue{}, fmt.Errorf("%w: number %d", ErrIssue, raw.Number)
	}
	title := sanitize(raw.Title)
	if utf8.RuneCountInString(title) == 0 || utf8.RuneCountInString(title) > 300 {
		return IntakeIssue{}, fmt.Errorf("%w: title length %d", ErrIssue, utf8.RuneCountInString(title))
	}
	body := sanitize(raw.Body)
	if utf8.RuneCountInString(body) > 20000 {
		return IntakeIssue{}, fmt.Errorf("%w: body too long", ErrIssue)
	}
	var state IssueState
	switch raw.State {
	case string(IssueOpen):
		state = IssueOpen
	case string(IssueClosed):
		state = IssueClosed
	default:
		return IntakeIssue{}, fmt.Errorf("%w: state %q", ErrIssue, raw.State)
	}
	author := sanitize(raw.Author)
	if utf8.RuneCountInString(author) == 0 || utf8.RuneCountInString(author) > 100 {
		return IntakeIssue{}, fmt.Errorf("%w: author", ErrIssue)
	}
	return IntakeIssue{
		ID:         workflow.WorkItemID(fmt.Sprintf("%s#%d", raw.Repository, raw.Number)),
		Repository: raw.Repository,
		Number:     raw.Number,
		Title:      title,
		Body:       body,
		State:      state,
		Author:     author,
	}, nil
}

func validRepository(repo string) bool {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 100 {
			return false
		}
		for _, r := range part {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
