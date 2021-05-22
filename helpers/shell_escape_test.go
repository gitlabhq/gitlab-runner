// +build !integration

package helpers

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkShellEscape(b *testing.B) {
	data := make([]byte, 1024*1024)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	dataStr := string(data)

	for i := 0; i < b.N; i++ {
		ShellEscape(dataStr)
	}
}

func TestShellEscape(t *testing.T) {
	var tests = []struct {
		in  string
		out string
	}{
		{"standard string", "$'standard string'"},
		{"+\t\n\r&", "$'+\\t\\n\\r&'"},
		{"", "''"},
	}

	for _, test := range tests {
		actual := ShellEscape(test.in)
		assert.Equal(t, test.out, actual, "src=%v", test.in)
	}
}
