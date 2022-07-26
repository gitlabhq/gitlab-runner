//go:build boringcrypto
// +build boringcrypto

package boring

import (
	"crypto/boring"
	"github.com/sirupsen/logrus"
)

func CheckBoring() {
	if boring.Enabled() {
		logrus.Warnln("FIPS mode enabled. Using BoringSSL.")
		return
	}

	logrus.Warnln("GitLab Runner was compiled with FIPS mode but BoringSSL is not enabled.")
}
