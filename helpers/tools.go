package helpers

// FirstNonZero gets the first value of the provided options that is not the type's zero value.
func FirstNonZero[T comparable](potentialValues ...T) (T, bool) {
	var zero T

	for _, val := range potentialValues {
		if val != zero {
			return val, true
		}
	}

	return zero, false
}
