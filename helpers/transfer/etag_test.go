//go:build !integration

package transfer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrongETag(t *testing.T) {
	tests := map[string]struct {
		etag string
		want bool
	}{
		"strong":               {etag: `"0123456789abcdef"`, want: true},
		"strong and empty":     {etag: `""`, want: true},
		"weak":                 {etag: `W/"0123456789abcdef"`, want: false},
		"none":                 {etag: ``, want: false},
		"unquoted":             {etag: `0123456789abcdef`, want: false},
		"opening quote only":   {etag: `"0123456789abcdef`, want: false},
		"a single quote":       {etag: `"`, want: false},
		"quote inside the tag": {etag: `"0123"4567"`, want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, StrongETag(tc.etag))
		})
	}
}
