// internal/delivery/release.go
package delivery

import (
	"fmt"
	"regexp"
)

var tagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type Artifact struct {
	Name   string
	Digest string
}

type Release struct {
	Tag       string
	Commit    string
	Artifacts []Artifact
}

func checkRelease(tag, commit string, artifacts []Artifact) error {
	if !tagPattern.MatchString(tag) {
		return fmt.Errorf("%w: tag %q is not vMAJOR.MINOR.PATCH", ErrDelivery, tag)
	}
	if !validSHA(commit) {
		return fmt.Errorf("%w: release commit must be a sha", ErrDelivery)
	}
	if len(artifacts) == 0 {
		return fmt.Errorf("%w: release needs at least one artifact", ErrDelivery)
	}
	for _, a := range artifacts {
		if a.Name == "" {
			return fmt.Errorf("%w: artifact without name", ErrDelivery)
		}
		if !digestPattern.MatchString(a.Digest) {
			return fmt.Errorf("%w: artifact %q needs a sha256 digest", ErrDelivery, a.Name)
		}
	}
	return nil
}

func CreateRelease(tag, commit string, artifacts []Artifact) (Release, error) {
	if err := checkRelease(tag, commit, artifacts); err != nil {
		return Release{}, err
	}
	return Release{Tag: tag, Commit: commit, Artifacts: append([]Artifact{}, artifacts...)}, nil
}

func VerifyRelease(r Release) error {
	return checkRelease(r.Tag, r.Commit, r.Artifacts)
}
