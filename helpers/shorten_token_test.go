//go:build !integration

package helpers

import (
	"testing"
)

func TestShortenToken(t *testing.T) {
	var tests = []struct {
		in  string
		out string
	}{
		{"short", "short"},
		{"veryverylongtoken", "veryvery"},
		{"GR1348941Z196cJVywzZpx_Ki_Cn2", "GR1348941Z196cJVy"},
		{"GJ1348941Z196cJVywzZpx_Ki_Cn2", "GJ134894"},
		{"GRveryverylongtoken", "GRveryve"},
		{"glrt-t9Wkyj-HGRkqQ-VWTGAr", "t9Wkyj-HG"},
	}

	for _, test := range tests {
		actual := ShortenToken(test.in)
		if actual != test.out {
			t.Error("Expected ", test.out, ", get ", actual)
		}
	}
}
