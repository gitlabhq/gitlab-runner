//go:build fips

package fips

import (
	"crypto/boring"

	"github.com/sirupsen/logrus"
)

// Enabled returns true if FIPS crypto has been enabled. For the FIPS Go
// compiler in https://github.com/golang-fips/go, this requires that:
//
//  1. The binary has been compiled with CGO_ENABLED=1.
//  2. The platform is amd64 running on a Linux runtime.
//  3. The kernel has FIPS enabled (e.g. `/proc/sys/crypto/fips_enabled` is 1).
//  4. A system OpenSSL can be dynamically loaded via ldopen().
func Enabled() bool {
	return boring.Enabled()
}

// Check logs whether FIPS crypto is active in this binary.
func Check() {
	if Enabled() {
		logrus.Info("FIPS mode is enabled. Using an external SSL library.")
		return
	}

	logrus.Info("Binary was compiled with FIPS mode, but an external SSL library was not enabled.")
}
