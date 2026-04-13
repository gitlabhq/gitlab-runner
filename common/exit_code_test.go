//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeExitCode(t *testing.T) {
	tests := map[string]struct {
		input    int
		expected int
	}{
		"zero":                                            {input: 0, expected: 0},
		"positive unix exit code":                         {input: 1, expected: 1},
		"max unix exit code":                              {input: 255, expected: 255},
		"windows DWORD -1":                                {input: 4294967295, expected: -1},
		"windows access violation":                        {input: 3221225477, expected: -1073741819},
		"windows DLL not found":                           {input: 3221225781, expected: -1073741515},
		"max positive int32":                              {input: 2147483647, expected: 2147483647},
		"int32 min (0x80000000)":                          {input: 2147483648, expected: -2147483648},
		"negative one directly":                           {input: -1, expected: -1},
		"value above MaxUint32 truncates to zero":         {input: 4294967296, expected: 0},  // 0x1_00000000 → 0
		"value above MaxUint32 truncates to negative one": {input: 8589934591, expected: -1}, // 0x1_FFFFFFFF → -1
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeExitCode(tt.input))
		})
	}
}
