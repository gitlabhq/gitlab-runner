package archives

import (
	"fmt"
	"io"
	"os"

	gzip "github.com/klauspost/pgzip"
	"github.com/sirupsen/logrus"
)

func writeGzipFile(w io.Writer, fileName string, fileInfo os.FileInfo) error {
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("the %q is not a regular file", fileName)
	}

	gz := gzip.NewWriter(w)
	gz.Header.Name = fileInfo.Name()
	gz.Header.Comment = fileName
	gz.Header.ModTime = fileInfo.ModTime()
	defer func() { _ = gz.Close() }()

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(gz, file)
	return err
}

func CreateGzipArchive(w io.Writer, fileNames []string) error {
	for _, fileName := range fileNames {
		fi, err := os.Lstat(fileName)
		if os.IsNotExist(err) {
			logrus.Warningln("File ignored:", err)
			continue
		} else if err != nil {
			return err
		}

		err = writeGzipFile(w, fileName, fi)
		if err != nil {
			return err
		}
	}

	return nil
}
