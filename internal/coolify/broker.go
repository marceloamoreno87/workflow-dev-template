// internal/coolify/broker.go
package coolify

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

var ErrCoolify = errors.New("invalid coolify request")

var resourcePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

var tagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type Broker struct {
	baseURL string
	token   string
}

func NewBroker(baseURL, token string) (Broker, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Broker{}, fmt.Errorf("%w: base url needs http(s) with host", ErrCoolify)
	}
	if u.User != nil {
		return Broker{}, fmt.Errorf("%w: credentials do not belong in urls", ErrCoolify)
	}
	if token == "" {
		return Broker{}, fmt.Errorf("%w: token required", ErrCoolify)
	}
	return Broker{baseURL: u.Scheme + "://" + u.Host + u.Path, token: token}, nil
}

type DeployRequest struct {
	ResourceID     string
	ReleaseTag     string
	ReleaseCommit  string
	FeatureEnabled bool
}

func (r DeployRequest) Validate() error {
	if !resourcePattern.MatchString(r.ResourceID) {
		return fmt.Errorf("%w: resource id", ErrCoolify)
	}
	if !tagPattern.MatchString(r.ReleaseTag) {
		return fmt.Errorf("%w: release tag %q", ErrCoolify, r.ReleaseTag)
	}
	if !shaPattern.MatchString(r.ReleaseCommit) {
		return fmt.Errorf("%w: release commit", ErrCoolify)
	}
	if r.FeatureEnabled {
		return fmt.Errorf("%w: deploy must land with the feature disabled", ErrCoolify)
	}
	return nil
}

type RollbackRequest struct {
	ResourceID     string
	ToDeploymentID string
	Reason         string
}

func (r RollbackRequest) Validate() error {
	if !resourcePattern.MatchString(r.ResourceID) {
		return fmt.Errorf("%w: resource id", ErrCoolify)
	}
	if !resourcePattern.MatchString(r.ToDeploymentID) {
		return fmt.Errorf("%w: deployment id", ErrCoolify)
	}
	if r.Reason == "" {
		return fmt.Errorf("%w: rollback reason required", ErrCoolify)
	}
	if len([]rune(r.Reason)) > 2000 {
		return fmt.Errorf("%w: rollback reason too long", ErrCoolify)
	}
	return nil
}
