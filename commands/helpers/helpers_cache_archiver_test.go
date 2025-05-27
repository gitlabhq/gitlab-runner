// Helper functions that are shared between unit tests and integration tests

package helpers

import (
	"os"
	"time"

	"gocloud.dev/blob"
)

// NewCacheArchiverCommandForTest exposes CacheArchiverCommand with fileArchiver to integration tests
func NewCacheArchiverCommandForTest(file string, fileArchiverPaths []string) CacheArchiverCommand {
	return CacheArchiverCommand{
		File:         file,
		fileArchiver: fileArchiver{Paths: fileArchiverPaths},
	}
}

func GetMatches(cmd *CacheArchiverCommand) map[string]os.FileInfo {
	return cmd.files
}

// SetCacheArchiverCommandMux allows integration tests to set mux
func SetCacheArchiverCommandMux(cmd *CacheArchiverCommand, mux *blob.URLMux) {
	cmd.mux = mux
}

// SetCacheArchiverCommandClientTimeout allows integration tests to set the client timeout
func SetCacheArchiverCommandClientTimeout(cmd *CacheArchiverCommand, timeout time.Duration) {
	cmd.getClient().Timeout = timeout
}
