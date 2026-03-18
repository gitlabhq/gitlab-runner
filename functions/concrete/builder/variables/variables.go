package variables

import "strconv"

type Provider interface {
	Get(string) string
	ExpandValue(string) string
}

// Default returns the variable value for key, or defaultValue if unset.
// If allowedValues are provided, the value must be one of them or defaultValue is returned.
func Default(v Provider, key, defaultValue string, allowedValues ...string) string {
	if s := v.Get(key); s != "" {
		if len(allowedValues) == 0 {
			return s
		}
		for _, allowed := range allowedValues {
			if s == allowed {
				return s
			}
		}
	}
	return defaultValue
}

// DefaultBool parses a bool from the variable key, returning defaultValue
// if the key is unset or unparseable.
func DefaultBool(v Provider, key string, defaultValue bool) bool {
	val, err := strconv.ParseBool(v.Get(key))
	if err != nil {
		return defaultValue
	}
	return val
}

// DefaultIntClamp returns an int parsed from the variable key, clamped to
// [lo, hi]. Returns the clamped defaultValue if the key is unset or unparseable.
func DefaultIntClamp(v Provider, key string, defaultValue, lo, hi int) int {
	val, err := strconv.Atoi(v.Get(key))
	if err != nil {
		val = defaultValue
	}
	return min(max(val, lo), hi)
}
