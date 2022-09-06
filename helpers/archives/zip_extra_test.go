//go:build !integration

package archives

import (
	"os"
	"runtime"
	"testing"

	"archive/zip"
	"encoding/binary"

	"github.com/stretchr/testify/assert"
)

func TestCreateZipExtra(t *testing.T) {
	testFile, err := os.CreateTemp("", "test")
	assert.NoError(t, err)
	defer testFile.Close()
	defer os.Remove(testFile.Name())

	fi, _ := testFile.Stat()
	assert.NotNil(t, fi)

	data := createZipExtra(fi)
	size := binary.Size(&ZipExtraField{})*2 +
		binary.Size(&ZipUIDGidField{}) +
		binary.Size(&ZipTimestampField{})

	// windows only support the timestamp extra field
	if runtime.GOOS == "windows" {
		size = binary.Size(&ZipExtraField{}) +
			binary.Size(&ZipTimestampField{})
	}

	assert.Equal(t, len(data), size)
}

func TestProcessZipExtra(t *testing.T) {
	testFile, err := os.CreateTemp("", "test")
	assert.NoError(t, err)
	defer testFile.Close()
	defer os.Remove(testFile.Name())

	fi, _ := testFile.Stat()
	assert.NotNil(t, fi)

	zipFile, err := zip.FileInfoHeader(fi)
	assert.NoError(t, err)
	zipFile.Extra = createZipExtra(fi)

	err = os.WriteFile(fi.Name(), []byte{}, 0o666)
	defer os.Remove(fi.Name())
	assert.NoError(t, err)

	err = processZipExtra(zipFile)
	assert.NoError(t, err)

	fi2, _ := testFile.Stat()
	assert.NotNil(t, fi2)
	assert.Equal(t, fi.Mode(), fi2.Mode())
	assert.Equal(t, fi.ModTime(), fi2.ModTime())
}
