// internal/knowledge/concept.go
package knowledge

import (
	"errors"
	"fmt"
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
	ID      string
	Title   string
	Version int
	Status  string
	Tags    []string
	Body    string
	Path    string
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
	}
	return bundle, nil
}
