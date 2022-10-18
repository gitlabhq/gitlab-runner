//go:build !integration

package trace

import (
	"bytes"
	"io"
	"math"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestRandomCopyReadback(t *testing.T) {
	input := make([]byte, 1*1024*1024)
	_, err := rand.Read(input)
	require.NoError(t, err)

	input = bytes.ToValidUTF8(input, []byte(string(utf8.RuneError)))

	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetLimit(math.MaxInt64)
	buffer.SetMasked(common.MaskOptions{
		Phrases: []string{"a"},
	})

	n, err := io.Copy(buffer, bytes.NewReader(input))
	require.NoError(t, err)
	require.Equal(t, n, int64(len(input)))

	buffer.Finish()

	content, err := buffer.Bytes(0, math.MaxInt64)
	require.NoError(t, err)

	expected := strings.ReplaceAll(string(input), "a", "[MASKED]")

	assert.Equal(t, []byte(expected), content)
}
