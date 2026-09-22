// internal/flags/flag.go
package flags

import (
	"errors"
	"fmt"
	"regexp"
)

var ErrFlag = errors.New("invalid feature flag")

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

var attributePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

type TargetRule struct {
	Name       string
	Attribute  string
	Values     []string
	Percentage float64
	Variation  bool
}

type Flag struct {
	Key        string
	Enabled    bool
	Rules      []TargetRule
	DefaultOn  bool
	Percentage float64
}

func validPercentage(p float64) bool {
	return p >= 0 && p <= 100
}

func (f Flag) Validate() error {
	if !keyPattern.MatchString(f.Key) {
		return fmt.Errorf("%w: key", ErrFlag)
	}
	if len(f.Rules) > 16 {
		return fmt.Errorf("%w: too many rules", ErrFlag)
	}
	for _, r := range f.Rules {
		if r.Name == "" || len([]rune(r.Name)) > 128 {
			return fmt.Errorf("%w: rule name", ErrFlag)
		}
		if !attributePattern.MatchString(r.Attribute) {
			return fmt.Errorf("%w: attribute %q", ErrFlag, r.Attribute)
		}
		if len(r.Values) == 0 || len(r.Values) > 64 {
			return fmt.Errorf("%w: rule %q needs 1..64 values", ErrFlag, r.Name)
		}
		for _, v := range r.Values {
			if n := len([]rune(v)); n == 0 || n > 256 {
				return fmt.Errorf("%w: value length %d", ErrFlag, n)
			}
		}
		if !validPercentage(r.Percentage) {
			return fmt.Errorf("%w: percentage %v", ErrFlag, r.Percentage)
		}
	}
	if !validPercentage(f.Percentage) {
		return fmt.Errorf("%w: default percentage %v", ErrFlag, f.Percentage)
	}
	return nil
}
