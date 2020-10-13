package archives

import (
	"archive/zip"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

func createZipDirectoryEntry(archive *zip.Writer, fh *zip.FileHeader) error {
	fh.Name += "/"
	_, err := archive.CreateHeader(fh)
	return err
}

func createZipSymlinkEntry(archive *zip.Writer, fh *zip.FileHeader) error {
	fw, err := archive.CreateHeader(fh)
	if err != nil {
		return err
	}

	link, err := os.Readlink(fh.Name)
	if err != nil {
		return err
	}

	_, err = io.WriteString(fw, link)
	return err
}

func createZipFileEntry(archive *zip.Writer, fh *zip.FileHeader) error {
	fh.Method = zip.Deflate
	fw, err := archive.CreateHeader(fh)
	if err != nil {
		return err
	}

	file, err := os.Open(fh.Name)
	if err != nil {
		return err
	}

	_, err = io.Copy(fw, file)
	_ = file.Close()
	if err != nil {
		return err
	}
	return nil
}

func createZipEntry(archive *zip.Writer, fileName string) error {
	fi, err := os.Lstat(fileName)
	if err != nil {
		logrus.Warningln("File ignored:", err)
		return nil
	}

	fh, err := zip.FileInfoHeader(fi)
	if err != nil {
		return err
	}
	fh.Name = fileName
	fh.Extra = createZipExtra(fi)
	// Set EFS flag to indicate that filenames and comments are UTF-8 encoded
	fh.Flags |= 0x800

	switch fi.Mode() & os.ModeType {
	case os.ModeDir:
		return createZipDirectoryEntry(archive, fh)

	case os.ModeSymlink:
		return createZipSymlinkEntry(archive, fh)

	case os.ModeNamedPipe, os.ModeSocket, os.ModeDevice:
		// Ignore the files that of these types
		logrus.Warningln("File ignored:", fileName)
		return nil

	default:
		return createZipFileEntry(archive, fh)
	}
}

func CreateZipArchive(w io.Writer, fileNames []string) error {
	tracker := newPathErrorTracker()

	archive := zip.NewWriter(w)
	defer func() { _ = archive.Close() }()

	for _, fileName := range fileNames {
		if err := errorIfGitDirectory(fileName); tracker.actionable(err) {
			printGitArchiveWarning("archive")
		}

		err := createZipEntry(archive, fileName)
		if err != nil {
			return err
		}
	}

	return nil
}
