//go:build !integration

package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShortenToken(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// no prefix
		{"short", "short"},
		{"veryverylongtoken", "veryveryl"},

		// partition prefix only
		{"t1_t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"t2_t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"t3_t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"t4_t9Wkyj-HGRkqQ-VWTGAr", "t4_t9Wkyj"},

		// glrt prefix, with and without partition prefix
		{"glrt-t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"glrt-t1_t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},

		// glrtr prefix, with and without partition prefix, though the latter should never happen
		{"glrtr-t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"glrtr-t1_t9Wkyj-HGRkqQ-VWTGAr", "t1_t9Wkyj"},

		// glcbt prefix, with and without partition prefix, though the latter should never happen
		{"glcbt-t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"glcbt-t2_t9Wkyj-HGRkqQ-VWTGAr", "t2_t9Wkyj"},

		// old registration token, with and without 7 char decimal-to-hex-encoded rotation date
		{"GR1348941Z196cJVywzZpx_Ki_Cn2", "Z196cJVyw"},
		{"GR134894-196cJVywzZpx_Ki_Cn2", "GR134894-"},
	}

	for _, test := range tests {
		assert.Equal(t, test.want, ShortenToken(test.in))
	}
}
