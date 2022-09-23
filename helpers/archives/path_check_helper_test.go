//go:build !integration

package archives

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoesPathsListContainGitDirectory(t *testing.T) {
	examples := []struct {
		path   string
		unsafe bool
	}{
		{".git", true},
		{".git/", true},
		{"././././.git/", true},
		{"././.git/.././.git/", true},
		{".git/test", true},
		{"./.git/test", true},
		{"test/.git", false},
		{"test/.git/test", false},
	}

	for id, example := range examples {
		t.Run(fmt.Sprintf("example-%d", id), func(t *testing.T) {
			unsafe := isPathAGitDirectory(example.path)
			assert.Equal(t, example.unsafe, unsafe)
		})
	}
}
