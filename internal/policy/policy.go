package policy

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"quwin/api-gateway/internal/limiter"
)

type Store struct {
	defaultPolicy limiter.Policy
	byPlan        map[string]limiter.Policy
}

func NewStore(defaultPolicy limiter.Policy, byPlan map[string]limiter.Policy) *Store {
	return &Store{
		defaultPolicy: defaultPolicy,
		byPlan:        byPlan,
	}
}

func (s *Store) ForPlan(plan string) limiter.Policy {
	if policy, ok := s.byPlan[plan]; ok {
		return policy
	}
	return s.defaultPolicy
}

// ParsePlanPolicies parses:
// RATE_LIMIT_POLICIES="free:5:1m:5,pro:60:1m:60,enterprise:600:1m:600"
//
// Format:
// plan:limit:window:capacity
//
// refillRate is derived as limit / window.Seconds().
func ParsePlanPolicies(value string) (map[string]limiter.Policy, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return map[string]limiter.Policy{}, nil
	}

	records := strings.Split(value, ",")
	policies := make(map[string]limiter.Policy, len(records))

	for _, record := range records {
		parts := strings.Split(record, ":")
		if len(parts) != 4 {
			return nil, fmt.Errorf("invalid policy %q; expected plan:limit:window:capacity", record)
		}

		plan := strings.TrimSpace(parts[0])
		if plan == "" {
			return nil, fmt.Errorf("invalid policy %q; plan is required", record)
		}

		limit, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil || limit <= 0 {
			return nil, fmt.Errorf("invalid limit in policy %q", record)
		}

		window, err := time.ParseDuration(strings.TrimSpace(parts[2]))
		if err != nil || window <= 0 {
			return nil, fmt.Errorf("invalid window in policy %q", record)
		}

		capacity, err := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
		if err != nil || capacity <= 0 {
			return nil, fmt.Errorf("invalid capacity in policy %q", record)
		}

		policies[plan] = limiter.Policy{
			Limit:      limit,
			Window:     window,
			Capacity:   capacity,
			RefillRate: float64(limit) / window.Seconds(),
		}
	}

	return policies, nil
}