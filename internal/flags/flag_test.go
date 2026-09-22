package flags

import (
	"testing"
)

func validFlag() Flag {
	return Flag{
		Key:     "checkout.redesign",
		Enabled: true,
		Rules: []TargetRule{
			{Name: "team", Attribute: "email", Values: []string{"@example.com"}, Percentage: 100, Variation: true},
		},
		DefaultOn:  false,
		Percentage: 10,
	}
}

func TestValidateFlag(t *testing.T) {
	t.Parallel()

	if err := validFlag().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadFlags(t *testing.T) {
	t.Parallel()

	mk := func(mut func(*Flag)) Flag {
		flag := validFlag()
		mut(&flag)
		return flag
	}
	for _, tc := range []struct {
		name string
		flag Flag
	}{
		{name: "empty key", flag: mk(func(f *Flag) { f.Key = "" })},
		{name: "bad key chars", flag: mk(func(f *Flag) { f.Key = "my flag!" })},
		{name: "too many rules", flag: mk(func(f *Flag) {
			f.Rules = make([]TargetRule, 17)
			for i := range f.Rules {
				f.Rules[i] = TargetRule{Name: "r", Attribute: "email", Values: []string{"a"}}
			}
		})},
		{name: "empty rule name", flag: mk(func(f *Flag) {
			f.Rules = []TargetRule{{Attribute: "email", Values: []string{"a"}}}
		})},
		{name: "bad attribute", flag: mk(func(f *Flag) {
			f.Rules = []TargetRule{{Name: "r", Attribute: "e-mail", Values: []string{"a"}}}
		})},
		{name: "no values", flag: mk(func(f *Flag) {
			f.Rules = []TargetRule{{Name: "r", Attribute: "email"}}
		})},
		{name: "bad percentage", flag: mk(func(f *Flag) {
			f.Rules = []TargetRule{{Name: "r", Attribute: "email", Values: []string{"a"}, Percentage: 101}}
		})},
		{name: "bad default percentage", flag: mk(func(f *Flag) { f.Percentage = -1 })},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.flag.Validate(); err == nil {
				t.Fatal("expected rejection, got none")
			}
		})
	}
}
