// Package mathutil provides generic math utilities.
package mathutil

import "cmp"

// Clamp returns v clamped to the range [lo, hi].
// If lo > hi, Clamp returns hi.
func Clamp[T cmp.Ordered](v, lo, hi T) T {
	return min(max(v, lo), hi)
}
