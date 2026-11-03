//go:build integration

package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The GoCloud path signs Range in the SigV4 header signature, so a proxy cannot remove it. The
// second node of this load balancer is a static server that ignores Range itself, and answers a
// chunk request with 200 and the whole object. The download then falls back to a single reader,
// which gets the whole object from either node.
func TestGoCloudParallelDownloadRangeIgnoringLoadBalancer(t *testing.T) {
	store := newRangeITStore(t)
	lb := rangeITEnv(t, "RANGE_IT_STATIC_LB")
	dir := rangeITEnv(t, "RANGE_IT_STATIC_DIR")
	downloaded := testGoCloudParallelDownload(t, store, lb, func(key string, payload []byte) {
		path := filepath.Join(dir, rangeITBucket, key)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, payload, 0o644))
		t.Cleanup(func() { _ = os.Remove(path) })
	})
	require.True(t, downloaded, "archive should be downloaded")
}
