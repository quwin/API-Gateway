package limiter_test

import (
	"testing"
	"time"

	"quwin/api-gateway/internal/limiter"
	"quwin/api-gateway/internal/policy"
)

func TestParsePlanPoliciesParsesValidPolicies(t *testing.T) {
	got, err := policy.ParsePlanPolicies("free:5:1m:5,pro:60:1m:60")
	if err != nil {
		t.Fatal(err)
	}

	free := got["free"]
	if free.Limit != 5 {
		t.Fatalf("free limit: expected 5, got %d", free.Limit)
	}
	if free.Window != time.Minute {
		t.Fatalf("free window: expected 1m, got %s", free.Window)
	}
	if free.Capacity != 5 {
		t.Fatalf("free capacity: expected 5, got %d", free.Capacity)
	}
	if free.RefillRate != float64(5)/60.0 {
		t.Fatalf("free refill rate: expected %f, got %f", float64(5)/60.0, free.RefillRate)
	}

	pro := got["pro"]
	if pro.Limit != 60 {
		t.Fatalf("pro limit: expected 60, got %d", pro.Limit)
	}
	if pro.Window != time.Minute {
		t.Fatalf("pro window: expected 1m, got %s", pro.Window)
	}
	if pro.Capacity != 60 {
		t.Fatalf("pro capacity: expected 60, got %d", pro.Capacity)
	}
	if pro.RefillRate != 1 {
		t.Fatalf("pro refill rate: expected 1, got %f", pro.RefillRate)
	}
}

func TestParsePlanPoliciesReturnsEmptyMapForEmptyInput(t *testing.T) {
	got, err := policy.ParsePlanPolicies("")
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 0 {
		t.Fatalf("expected empty map, got %+v", got)
	}
}

func TestParsePlanPoliciesRejectsInvalidRecords(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "missing capacity", value: "free:5:1m"},
		{name: "empty plan", value: ":5:1m:5"},
		{name: "zero limit", value: "free:0:1m:5"},
		{name: "negative limit", value: "free:-1:1m:5"},
		{name: "invalid limit", value: "free:nope:1m:5"},
		{name: "zero window", value: "free:5:0s:5"},
		{name: "negative window", value: "free:5:-1m:5"},
		{name: "invalid window", value: "free:5:nope:5"},
		{name: "zero capacity", value: "free:5:1m:0"},
		{name: "negative capacity", value: "free:5:1m:-1"},
		{name: "invalid capacity", value: "free:5:1m:nope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := policy.ParsePlanPolicies(tt.value)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestStoreReturnsPlanPolicyWhenPresent(t *testing.T) {
	defaultPolicy := limiter.Policy{
		Limit:      5,
		Window:     time.Minute,
		Capacity:   5,
		RefillRate: float64(5) / 60,
	}

	proPolicy := limiter.Policy{
		Limit:      60,
		Window:     time.Minute,
		Capacity:   60,
		RefillRate: 1,
	}

	store := policy.NewStore(defaultPolicy, map[string]limiter.Policy{
		"pro": proPolicy,
	})

	got := store.ForPlan("pro")

	if got != proPolicy {
		t.Fatalf("expected pro policy %+v, got %+v", proPolicy, got)
	}
}

func TestStoreReturnsDefaultPolicyWhenPlanMissing(t *testing.T) {
	defaultPolicy := limiter.Policy{
		Limit:      5,
		Window:     time.Minute,
		Capacity:   5,
		RefillRate: float64(5) / 60,
	}

	store := policy.NewStore(defaultPolicy, nil)

	got := store.ForPlan("unknown")

	if got != defaultPolicy {
		t.Fatalf("expected default policy %+v, got %+v", defaultPolicy, got)
	}
}