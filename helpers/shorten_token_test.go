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

		// instance token prefix before glrt and glcbt; GitLab never adds one to glrtr
		// (see Ci::Runner#prefix_for_new_and_legacy_runner)
		{"acme-glrt-t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"acme-glrt-t1_t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"Acme-glrt-t2_t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"Acme1-glrt-t3_t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"acme-glcbt-t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"123-glrt-t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
		{"acme-glrtr-t9Wkyj-HGRkqQ-VWTGAr", "acme-glrt"},

		// old registration token, with and without 7 char decimal-to-hex-encoded rotation date
		{"GR1348941Z196cJVywzZpx_Ki_Cn2", "Z196cJVyw"},
		{"GR134894-196cJVywzZpx_Ki_Cn2", "GR134894r"}, // trailing dash replaced with "r"

		// leading non-alphanumeric after prefix strip: "r" is prepended and the
		// original leading character is preserved at position 1 (no entropy loss).
		{"glrt--abcdefghijk", "r-abcdefg"},  // dash after glrt- strip
		{"glcbt--abcdefghijk", "r-abcdefg"}, // dash after glcbt- strip
		{"-abcdefghijk", "r-abcdefg"},       // bare leading dash, no prefix
		{"_abcdefghijk", "r_abcdefg"},       // leading underscore, no prefix
		{".abcdefghijk", "r.abcdefg"},       // leading dot, no prefix

		// trailing non-alphanumeric: last character replaced with "r".
		{"abcdefgh-", "abcdefghr"}, // trailing dash
		{"abcdefgh_", "abcdefghr"}, // trailing underscore

		// short input (fewer than 9 chars): no panic, leading/trailing both fixed.
		{"_x", "rr"},     // "_x": leading _→r gives "r_", trailing _→r gives "rr"
		{"-abc", "r-ab"}, // "-abc": leading -→r gives "r-ab", trailing b is alphanumeric
	}

	for _, test := range tests {
		assert.Equal(t, test.want, ShortenToken(test.in))
	}
}
