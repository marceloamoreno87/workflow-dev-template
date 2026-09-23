// internal/githublive/poll.go
package githublive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ownerPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

type Issue struct {
	Number    int
	Title     string
	State     string
	UpdatedAt time.Time
}

type Poller struct {
	client Client
	owner  string
	repo   string
}

func NewPoller(client Client, owner, repo string) (*Poller, error) {
	if !ownerPattern.MatchString(owner) || !ownerPattern.MatchString(repo) {
		return nil, fmt.Errorf("%w: owner/repo", ErrGitHub)
	}
	return &Poller{client: client, owner: owner, repo: repo}, nil
}

func (p *Poller) Poll(ctx context.Context, since time.Time, limit int) ([]Issue, error) {
	if limit <= 0 || limit > 100 {
		return nil, fmt.Errorf("%w: limit outside 1..100", ErrGitHub)
	}
	query := url.Values{}
	query.Set("state", "all")
	query.Set("per_page", strconv.Itoa(limit))
	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}
	body, _, err := p.client.get(ctx, "/repos/"+p.owner+"/"+p.repo+"/issues", query.Encode())
	if err != nil {
		return nil, err
	}
	var docs []struct {
		Number    int     `json:"number"`
		Title     string  `json:"title"`
		Body      *string `json:"body"`
		State     string  `json:"state"`
		UpdatedAt string  `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &docs); err != nil {
		return nil, fmt.Errorf("%w: decode issues", ErrGitHub)
	}
	issues := make([]Issue, 0, len(docs))
	for _, doc := range docs {
		title := strings.TrimSpace(doc.Title)
		if doc.Number <= 0 || title == "" {
			return nil, fmt.Errorf("%w: malformed issue", ErrGitHub)
		}
		if doc.State != "open" && doc.State != "closed" {
			return nil, fmt.Errorf("%w: issue state %q", ErrGitHub, doc.State)
		}
		var updated time.Time
		if doc.UpdatedAt != "" {
			parsed, err := time.Parse(time.RFC3339, doc.UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("%w: issue timestamp", ErrGitHub)
			}
			updated = parsed
		}
		issues = append(issues, Issue{Number: doc.Number, Title: title, State: doc.State, UpdatedAt: updated})
	}
	return issues, nil
}
