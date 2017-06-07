package docker_helpers

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceLogger(t *testing.T) {
	output := new(bytes.Buffer)

	logger := NewServiceLogger("prefix", output)
	input := logger.Writer()

	fmt.Fprint(input, "test line 1\n")
	fmt.Fprint(input, "test line 2\n")

	actualOutput := output.String()
	assert.Regexp(t, ".*\\[prefix\\].* test line 1", actualOutput)
	assert.Regexp(t, ".*\\[prefix\\].* test line 2", actualOutput)
}
