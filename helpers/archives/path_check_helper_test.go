package archives

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoesPathsListContainGitDirectory(t *testing.T) {
	examples := []struct {
		paths  []string
		unsafe bool
	}{
		{[]string{".git"}, true},
		{[]string{".git/"}, true},
		{[]string{"././././.git/"}, true},
		{[]string{"././.git/.././.git/"}, true},
		{[]string{".git/test"}, true},
		{[]string{"./.git/test"}, true},
		{[]string{"test/.git"}, false},
		{[]string{"test/.git/test"}, false},
	}

	for id, example := range examples {
		t.Run(fmt.Sprintf("example-%d", id), func(t *testing.T) {
			unsafe := doesPathsListContainGitDirectory(example.paths)
			assert.Equal(t, example.unsafe, unsafe)
		})
	}
}
