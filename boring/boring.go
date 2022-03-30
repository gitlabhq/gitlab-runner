//go:build boringcrypto
// +build boringcrypto

package boring

import (
	"crypto/boring"
	"fmt"
)

func CheckBoring() {
	if boring.Enabled() {
		fmt.Println("FIPS mode enabled. Using BoringSSL.")
		return
	}

	fmt.Println("GitLab Runner was compiled with FIPS mode but BoringSSL is not enabled.")
}
