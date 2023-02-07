//go:build !integration

package helpers

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	artifactsTestArchivedFile  = "archive_file"
	artifactsTestArchivedFile2 = "archive_file2"
)

type testNetwork struct {
	common.MockNetwork
	downloadState        common.DownloadState
	downloadCalled       int
	directDownloadCalled int
	uploadState          common.UploadState
	uploadCalled         int
	uploadFormat         common.ArtifactFormat
	uploadName           string
	uploadType           string
	uploadedFiles        []string
}

func (m *testNetwork) DownloadArtifacts(
	config common.JobCredentials,
	artifactsFile io.WriteCloser,
	directDownload *bool,
) common.DownloadState {
	m.downloadCalled++

	if directDownload != nil && *directDownload {
		m.directDownloadCalled++
	}

	if m.downloadState == common.DownloadSucceeded {
		defer func() { _ = artifactsFile.Close() }()

		archive := zip.NewWriter(artifactsFile)
		_, _ = archive.Create(artifactsTestArchivedFile)
		_ = archive.Close()
	}
	return m.downloadState
}

func (m *testNetwork) consumeZipUpload(reader io.Reader) common.UploadState {
	var buffer bytes.Buffer
	_, _ = io.Copy(&buffer, reader)
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		logrus.Warningln(err)
		return common.UploadForbidden
	}

	for _, file := range archive.File {
		m.uploadedFiles = append(m.uploadedFiles, file.Name)
	}

	m.uploadFormat = common.ArtifactFormatZip

	return m.uploadState
}

func (m *testNetwork) consumeGzipUpload(reader io.Reader) common.UploadState {
	var buffer bytes.Buffer
	_, _ = io.Copy(&buffer, reader)

	stream := bytes.NewReader(buffer.Bytes())

	gz, err := gzip.NewReader(stream)
	gz.Multistream(false)
	if err != nil {
		logrus.Warningln("Invalid gzip stream")
		return common.UploadForbidden
	}

	// Read multiple streams
	for {
		_, err = io.Copy(io.Discard, gz)
		if err != nil {
			logrus.Warningln("Invalid gzip stream")
			return common.UploadForbidden
		}

		m.uploadedFiles = append(m.uploadedFiles, gz.Name)

		if gz.Reset(stream) == io.EOF {
			break
		}
		gz.Multistream(false)
	}

	m.uploadFormat = common.ArtifactFormatGzip

	return m.uploadState
}

func (m *testNetwork) consumeRawUpload(reader io.Reader) common.UploadState {
	_, err := io.Copy(io.Discard, reader)
	if err != nil {
		return common.UploadFailed
	}

	m.uploadedFiles = append(m.uploadedFiles, "raw")
	m.uploadFormat = common.ArtifactFormatRaw
	return m.uploadState
}

func (m *testNetwork) UploadRawArtifacts(
	config common.JobCredentials,
	reader io.ReadCloser,
	options common.ArtifactsOptions,
) (common.UploadState, string) {
	m.uploadCalled++

	if m.uploadState == common.UploadSucceeded {
		m.uploadType = options.Type
		m.uploadName = options.BaseName

		switch options.Format {
		case common.ArtifactFormatZip, common.ArtifactFormatDefault:
			return m.consumeZipUpload(reader), ""

		case common.ArtifactFormatGzip:
			return m.consumeGzipUpload(reader), ""

		case common.ArtifactFormatRaw:
			return m.consumeRawUpload(reader), ""

		default:
			return common.UploadForbidden, ""
		}
	}

	return m.uploadState, ""
}

func writeTestFile(t *testing.T, fileName string) {
	err := os.WriteFile(fileName, nil, 0o600)
	require.NoError(t, err, "Writing file:", fileName)
}

func removeTestFile(t *testing.T, fileName string) {
	err := os.Remove(fileName)
	require.NoError(t, err, "Removing file:", fileName)
}
