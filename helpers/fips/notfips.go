//go:build !fips

package fips

// Enabled returns false: this binary was not compiled with the fips build tag.
func Enabled() bool {
	return false
}

// Check does nothing: this binary was not compiled with the fips build tag.
func Check() {}
