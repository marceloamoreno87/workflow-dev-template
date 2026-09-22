// internal/flags/evaluate.go
package flags

import (
	"fmt"
	"hash/fnv"
	"strings"
)

type EvalContext struct {
	Attributes map[string]string
	BucketKey  string
}

func bucket(key, bucketKey string, percentage float64) bool {
	h := fnv.New64a()
	h.Write([]byte(key + "\x00" + bucketKey))
	return h.Sum64()%10000 < uint64(percentage*100)
}

func ruleMatches(rule TargetRule, ctx EvalContext) bool {
	value, ok := ctx.Attributes[rule.Attribute]
	if !ok {
		return false
	}
	for _, candidate := range rule.Values {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func (f Flag) Evaluate(ctx EvalContext) (bool, error) {
	if err := f.Validate(); err != nil {
		return false, err
	}
	if ctx.BucketKey == "" || len([]rune(ctx.BucketKey)) > 256 {
		return false, fmt.Errorf("%w: bucket key required", ErrFlag)
	}
	if len(ctx.Attributes) > 32 {
		return false, fmt.Errorf("%w: too many attributes", ErrFlag)
	}
	if !f.Enabled {
		return false, nil
	}
	for _, rule := range f.Rules {
		if ruleMatches(rule, ctx) {
			if rule.Percentage >= 100 {
				return rule.Variation, nil
			}
			if !bucket(f.Key+":"+rule.Name, ctx.BucketKey, rule.Percentage) {
				return !rule.Variation, nil
			}
			return rule.Variation, nil
		}
	}
	if f.Percentage >= 100 {
		return true, nil
	}
	if f.Percentage <= 0 && !f.DefaultOn {
		return false, nil
	}
	if bucket(f.Key, ctx.BucketKey, f.Percentage) {
		return true, nil
	}
	return f.DefaultOn, nil
}
