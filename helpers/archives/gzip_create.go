package archives

import (
	"compress/gzip"
	"io"
	"os"
)

func writeGzipFile(w io.Writer, fileName string, fileInfo os.FileInfo) error {
	gz := gzip.NewWriter(w)
	gz.Header.Name = fileInfo.Name()
	gz.Header.Comment = fileName
	gz.Header.ModTime = fileInfo.ModTime()
	defer gz.Close()

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(gz, file)
	return err
}

func CreateGzipArchive(w io.Writer, fileNames []string) error {
	for _, fileName := range fileNames {
		fi, err := os.Stat(fileName)
		if err != nil {
			return err
		}

		err = writeGzipFile(w, fileName, fi)
		if err != nil {
			return err
		}
	}

	return nil
}
