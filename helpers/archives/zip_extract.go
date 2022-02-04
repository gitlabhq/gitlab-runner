package archives

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func extractZipDirectoryEntry(file *zip.File) (err error) {
	err = os.Mkdir(file.Name, file.Mode().Perm())

	// The error that directory does exists is not a error for us
	if os.IsExist(err) {
		err = nil
	}
	return
}

func extractZipSymlinkEntry(file *zip.File) (err error) {
	var data []byte
	in, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	data, err = ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	// Remove symlink before creating a new one, otherwise we can error that file does exist
	_ = os.Remove(file.Name)
	err = os.Symlink(string(data), file.Name)
	return
}

func extractZipFileEntry(file *zip.File) (err error) {
	var out *os.File
	in, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	// Remove file before creating a new one, otherwise we can error that file does exist
	_ = os.Remove(file.Name)
	out, err = os.OpenFile(file.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)

	return
}

func extractZipFile(file *zip.File) (err error) {
	// Create all parents to extract the file
	err = os.MkdirAll(filepath.Dir(file.Name), 0777)
	if err != nil {
		return err
	}

	switch file.Mode() & os.ModeType {
	case os.ModeDir:
		err = extractZipDirectoryEntry(file)

	case os.ModeSymlink:
		err = extractZipSymlinkEntry(file)

	case os.ModeNamedPipe, os.ModeSocket, os.ModeDevice:
		// Ignore the files that of these types
		logrus.Warningf("File ignored: %q", file.Name)

	default:
		err = extractZipFileEntry(file)
	}
	return
}

// this string is consistently used across platforms
// see https://github.com/golang/go/blob/go1.17.6/src/syscall/zerrors_linux_386.go#L1393 and other zerrors*.go
const errCannotAllocateMemory = "cannot allocate memory"
const memoryErrorRetries = 3
const memoryErrorWaitTime = time.Second

func ExtractZipArchive(archive *zip.Reader) error {
	tracker := newPathErrorTracker()

	for _, file := range archive.File {
		if err := errorIfGitDirectory(file.Name); tracker.actionable(err) {
			printGitArchiveWarning("extract")
		}

		// we sometimes get memory errors unzipping, do a retry
		retries := memoryErrorRetries

		for {

			err := extractZipFile(file)
			if err == nil || !tracker.actionable(err) {
				break
			}

			logrus.Warningf("%s: %s (suppressing repeats)", file.Name, err)

			err, retries = checkMemoryAllocRetry(err, retries)

			if err != nil || retries <= 0 {
				return err
			}
		}
	}

	for _, file := range archive.File {
		// Update file permissions
		if err := os.Chmod(file.Name, file.Mode().Perm()); tracker.actionable(err) {
			logrus.Warningf("%s: %s (suppressing repeats)", file.Name, err)
		}

		// Process zip metadata
		if err := processZipExtra(&file.FileHeader); tracker.actionable(err) {
			logrus.Warningf("%s: %s (suppressing repeats)", file.Name, err)
		}
	}

	return nil
}

var disableWait = false // for testing

func checkMemoryAllocRetry(err error, retriesRemaining int) (error, int) {

	// When running in containers, we will occasionally get errors allocating
	// memory under pressure. So we retry a few times, then fail the extraction
	if err == nil || !strings.Contains(err.Error(), errCannotAllocateMemory) {
		return err, 0
	}

	if retriesRemaining <= 0 {
		return err, 0
	}

	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)

	logrus.Warningf(
		"Can't allocate memory, waiting %s, %d retries left (total used MB=%d)",
		memoryErrorWaitTime.String(),
		retriesRemaining,
		mem.Sys/1024/1024,
	)

	if !disableWait {
		time.Sleep(memoryErrorWaitTime)
	}
	retriesRemaining--
	return nil, retriesRemaining
}

func ExtractZipFile(fileName string) error {
	archive, err := zip.OpenReader(fileName)
	if err != nil {
		return err
	}
	defer func() { _ = archive.Close() }()

	return ExtractZipArchive(&archive.Reader)
}
