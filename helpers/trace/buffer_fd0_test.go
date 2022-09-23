//nolint:lll
//go:build !integration && (aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris)

package trace

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"

	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBufferHandlingWithExceededFDIssue(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Can be run only with root permissions")
	}

	fmt.Println("PID:", os.Getpid())

	fs, err := procfs.NewDefaultFS()
	require.NoError(t, err)

	proc, err := fs.Proc(os.Getpid())
	require.NoError(t, err)

	fff, _ := proc.FileDescriptors()
	t.Logf("%#v", fff)
	fds, err := proc.FileDescriptorTargets()
	require.NoError(t, err, "counting initial number of FDs")

	t.Logf("Initial file descriptors: %#v", fds)

	initialFDCount := len(fds)
	t.Log("Initial FDs count:", initialFDCount)

	additionalFDs := 100
	t.Log("Additional FDs count:", additionalFDs)

	maxFDsCount := initialFDCount + additionalFDs
	t.Log("Max FDs count:", maxFDsCount)

	var closeAtFinish []io.Closer

	defer setNewRLimitNOFILE(t, maxFDsCount)()
	defer closeClosers(t, closeAtFinish)

	assertFileDescriptors(t, proc, initialFDCount)

	filesToCreate := additionalFDs - 2
	t.Log("files to create", filesToCreate)
	for i := 0; i < filesToCreate; i++ {
		t.Log("loop: ", i+1)
		f, err := createNewLogFile(t)
		require.NoError(t, err, "try %d", i)
		require.NotNil(t, f, "try %d", i)

		closeAtFinish = append(closeAtFinish, f)
	}

	assertFileDescriptors(t, proc, initialFDCount+filesToCreate)

	ctx, cancelFn := context.WithCancel(context.Background())
	wg := new(sync.WaitGroup)
	wg.Add(1)

	runSleepProcess(ctx, wg, t)

	assertFileDescriptors(t, proc, initialFDCount+filesToCreate)

	file, err := createNewLogFile(t)
	closeAtFinish = append(closeAtFinish, file)
	assert.NoError(t, err)
	assert.NotNil(t, file)

	assertFileDescriptors(t, proc, initialFDCount+filesToCreate+1)

	closeClosers(t, closeAtFinish)
	closeAtFinish = make([]io.Closer, 0)

	assertFileDescriptors(t, proc, initialFDCount)

	maxAllowedFilesToCreate := additionalFDs
	t.Log("max allowed files to create:", maxAllowedFilesToCreate)
	for j := 0; j < maxAllowedFilesToCreate; j++ {
		t.Log("loop: ", j+1)
		file2, err := createNewLogFile(t)
		closeAtFinish = append(closeAtFinish, file2)
		assert.NoError(t, err, "try %d", j)
		if assert.NotNil(t, file2, "try %d", j) {
			assert.NotEqual(t, 0, file2.Fd(), "try %d", j)
		}
	}

	assertFileDescriptors(t, proc, initialFDCount+maxAllowedFilesToCreate)

	// Allocating the last free FD that was left for the previous
	// assertFileDescriptors() call
	file3, err := createNewLogFile(t)
	closeAtFinish = append(closeAtFinish, file3)
	require.NoError(t, err)

	file4, err := createNewLogFile(t)
	closeAtFinish = append(closeAtFinish, file4)
	assert.Nil(t, file4)
	assert.ErrorIs(t, err, syscall.EMFILE)

	closeClosers(t, closeAtFinish)

	assertFileDescriptors(t, proc, initialFDCount)

	cancelFn()
	wg.Wait()
}

func assertFileDescriptors(t *testing.T, proc procfs.Proc, expectedLen int) {
	targets, err := proc.FileDescriptorTargets()
	if !assert.NoError(t, err, "requesting FD targets") {
		return
	}

	fdsCount := len(targets)

	t.Logf("current FDs (%d): %#v", fdsCount, targets)

	assert.Equal(t, expectedLen, fdsCount, "Checking number of FDs")
	assert.Equal(t, "/dev/null", targets[0], "Checking what is FD=0")
}

func setNewRLimitNOFILE(t *testing.T, maxFDsCount int) func() {
	var originalRLimit syscall.Rlimit

	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &originalRLimit)
	require.NoError(t, err, "Requesting current RLIMIT_NOFILE")

	cleanup := func() {
		err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &originalRLimit)
		require.NoError(t, err, "Restoring original RLIMIT_NOFILE")
	}

	t.Logf("Setting max FD limit to %d (%v)", maxFDsCount, uint64(maxFDsCount))

	var newRLimit syscall.Rlimit
	newRLimit.Max = uint64(maxFDsCount)
	newRLimit.Cur = uint64(maxFDsCount)

	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &newRLimit)
	require.NoError(t, err, "Updating RLIMIT_NOFILE")

	return cleanup
}

func createNewLogFile(t *testing.T) (*os.File, error) {
	file, err := newLogFile()
	if file == nil {
		t.Log("Couldn't create log file:", err)
	} else {
		t.Log("Created log file with FD:", file.Fd())
	}

	return file, err
}

func runSleepProcess(ctx context.Context, wg *sync.WaitGroup, t *testing.T) {
	t.Log("Starting sleep process")

	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", "for i in $(seq 1 60); do echo '.'; sleep 1; done")

	err := cmd.Start()
	t.Log("cmd.Start() err:", err)
	started := !assert.ErrorIs(t, err, syscall.EMFILE, "Starting process")

	go func() {
		defer wg.Done()

		if !started {
			return
		}

		err := cmd.Wait()
		t.Log("cmd.Wait() err:", err)
		if err != nil {
			fmt.Printf("process error: %v\n", err)
		}
	}()
}

func closeClosers(t *testing.T, closers []io.Closer) {
	t.Log("closing closers")

	for _, c := range closers {
		if c == nil {
			continue
		}

		_ = c.Close()
	}
}
