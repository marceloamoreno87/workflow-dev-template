package flags

import (
	"testing"
)

func TestEvaluateTargeting(t *testing.T) {
	t.Parallel()

	flag := validFlag()
	on, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "dev@example.com"}, BucketKey: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("matching rule at 100% should be on")
	}
	off, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "stranger@other.com"}, BucketKey: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	_ = off // percentage-dependent; asserted for stability below
	again, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "stranger@other.com"}, BucketKey: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if off != again {
		t.Fatal("evaluation is not deterministic")
	}
}

func TestDisabledIsAlwaysOff(t *testing.T) {
	t.Parallel()

	flag := validFlag()
	flag.Enabled = false
	on, err := flag.Evaluate(EvalContext{Attributes: map[string]string{"email": "dev@example.com"}, BucketKey: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("disabled flag evaluated on")
	}
}

func TestBucketBoundaries(t *testing.T) {
	t.Parallel()

	full := Flag{Key: "k", Enabled: true, Percentage: 100}
	for _, key := range []string{"a", "b", "user-999"} {
		on, err := full.Evaluate(EvalContext{BucketKey: key})
		if err != nil || !on {
			t.Fatalf("100%% should always be on: %v %v", on, err)
		}
	}
	none := Flag{Key: "k", Enabled: true, Percentage: 0}
	for _, key := range []string{"a", "b", "user-999"} {
		on, err := none.Evaluate(EvalContext{BucketKey: key})
		if err != nil || on {
			t.Fatalf("0%% should always be off: %v %v", on, err)
		}
	}
}

func TestRejectBadContexts(t *testing.T) {
	t.Parallel()

	flag := validFlag()
	if _, err := flag.Evaluate(EvalContext{BucketKey: ""}); err == nil {
		t.Fatal("expected empty bucket rejection, got none")
	}
	bad := validFlag()
	bad.Key = "bad key!"
	if _, err := bad.Evaluate(EvalContext{BucketKey: "u"}); err == nil {
		t.Fatal("expected invalid flag rejection, got none")
	}
}
