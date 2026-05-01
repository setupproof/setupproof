package duration

import (
	"fmt"
	"strconv"
	"strings"
)

const maxInt64 = 1<<63 - 1

func ParseMillis(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("timeout must not be empty")
	}

	unit := byte(0)
	last := trimmed[len(trimmed)-1]
	if last < '0' || last > '9' {
		unit = last
		trimmed = strings.TrimSpace(trimmed[:len(trimmed)-1])
	}

	if trimmed == "" {
		return 0, fmt.Errorf("timeout %q is invalid", value)
	}
	if len(trimmed) > 1 && trimmed[0] == '0' {
		return 0, fmt.Errorf("timeout %q must be a positive integer without leading zeroes", value)
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] < '0' || trimmed[i] > '9' {
			return 0, fmt.Errorf("timeout %q must be a positive integer with optional s, m, or h suffix", value)
		}
	}

	amount, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || amount <= 0 {
		return 0, fmt.Errorf("timeout %q must be positive", value)
	}

	var multiplier int64
	switch unit {
	case 0, 's':
		multiplier = 1000
	case 'm':
		multiplier = 60 * 1000
	case 'h':
		multiplier = 60 * 60 * 1000
	default:
		return 0, fmt.Errorf("timeout %q uses unsupported unit %q", value, string(unit))
	}
	if amount > maxInt64/multiplier {
		return 0, fmt.Errorf("timeout %q is too large", value)
	}
	return amount * multiplier, nil
}
