//go:build !integration && !windows

package hosted_runners

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONTableReplacement(t *testing.T) {
	newRow := BridgeInfo{
		Timestamp: "TEST",
		Version:   "TEST",
		CommitSHA: "TEST",
		Flavor:    "TEST",
	}

	input, err := os.Open(filepath.Join("testdata", "table.md"))
	require.NoError(t, err, "Opening table.md file")

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	fn := prepareReplaceFn(log, newRow)
	out, err := fn(input)
	assert.NoError(t, err)

	rewrittenContent, err := os.ReadFile(filepath.Join("testdata", "table_rewritten.md"))
	require.NoError(t, err)

	assert.Equal(t, string(rewrittenContent), out)
}
