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
