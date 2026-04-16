package helpers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/meter"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/transfer"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/log"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // Needed to register the Azure driver
	_ "gocloud.dev/blob/s3blob"    // Needed to register the AWS S3 driver
	"gocloud.dev/gcerrors"
)

type CacheExtractorCommand struct {
	retryHelper
	meter.TransferMeterCommand

	File       string `long:"file" description:"The file containing your cache artifacts"`
	URL        string `long:"url" description:"URL of remote cache resource"`
	GoCloudURL string `long:"gocloud-url" description:"Go Cloud URL of remote cache resource (requires credentials)"`
	Timeout    int    `long:"timeout" description:"Overall timeout for cache downloading request (in minutes)"`
	EnvFile    string `long:"env-file" description:"Filename containing environment variables to read"`

	// Transfer options (all backends: presigned S3, GoCloud S3/Azure/GCS).
	TransferBufferSize int `long:"transfer-buffer-size" env:"CACHE_TRANSFER_BUFFER_SIZE" description:"Buffer size in bytes for streaming cache download (default 4 MiB)"`
	// Parallel download (presigned or GoCloud) requires FF_USE_PARALLEL_CACHE_TRANSFER. Concurrency > 1 for parallel.
	ChunkSize   int `long:"chunk-size" env:"CACHE_CHUNK_SIZE" description:"Chunk size in bytes for parallel cache download when FF_USE_PARALLEL_CACHE_TRANSFER is enabled (default 16 MiB; 0 falls back to default)"`
	Concurrency int `long:"concurrency" env:"CACHE_CONCURRENCY" description:"Concurrent chunks for parallel cache transfer when FF_USE_PARALLEL_CACHE_TRANSFER is enabled (default 16; 0 or 1 = sequential for download)"`

	client *CacheClient
	mux    *blob.URLMux
}

func NewCacheExtractorCommand() cli.Command {
	return common.NewCommand(
		"cache-extractor",
		"download and extract cache artifacts (internal)",
		&CacheExtractorCommand{
			retryHelper: retryHelper{
				Retry:     2,
				RetryTime: time.Second,
			},
			TransferBufferSize: defaultCacheTransferBufferSize,
			ChunkSize:          defaultCacheChunkSize,
			Concurrency:        defaultCacheConcurrency,
		},
	)
}

// normalizeExtractorArgs applies defaults for transfer buffer and chunk size when unset (0), matching
// CacheArchiverCommand.normalizeArgs for those fields. Concurrency is intentionally not normalized to the
// default here: 0 or 1 mean sequential download (see presignedParallelDownloadEligible).
func (c *CacheExtractorCommand) normalizeExtractorArgs() {
	if c.TransferBufferSize == 0 {
		c.TransferBufferSize = defaultCacheTransferBufferSize
	}
	if c.ChunkSize == 0 {
		c.ChunkSize = defaultCacheChunkSize
	}
}

func (c *CacheExtractorCommand) getClient() *CacheClient {
	if c.client == nil {
		c.client = NewCacheClient(c.Timeout)
	}

	return c.client
}

func checkIfUpToDate(path string, resp *http.Response) (bool, time.Time) {
	date, _ := time.Parse(http.TimeFormat, resp.Header.Get("Last-Modified"))
	return isLocalCacheFileUpToDate(path, date), date
}

func isLocalCacheFileUpToDate(path string, date time.Time) bool {
	fi, _ := os.Lstat(path)
	return fi != nil && !date.After(fi.ModTime())
}

func getRemoteCacheSize(resp *http.Response) int64 {
	length, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
	if length <= 0 {
		return meter.UnknownTotalSize
	}

	return int64(length)
}

func (c *CacheExtractorCommand) download(_ int) error {
	err := os.MkdirAll(filepath.Dir(c.File), 0o700)
	if err != nil {
		return err
	}

	if c.GoCloudURL != "" {
		logrus.Infoln("Using GoCloud URL for cache download")
		return c.handleGoCloudURL()
	}
	logrus.Infoln("Using presigned URL for cache download")
	return c.handlePresignedURL()
}

func (c *CacheExtractorCommand) getCache() (*http.Response, error) {
	resp, err := c.getClient().Get(c.URL)
	if err != nil {
		return nil, retryableErr{err: err}
	}

	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		return nil, os.ErrNotExist
	}

	return resp, retryOnServerError(resp)
}

// goCloudURLSchemeAssumesRangeSupport reports whether the Go CDK blob driver for this URL scheme
// is expected to support NewRangeReader without a per-download probe (S3, GCS, Azure Blob).
// Custom or test schemes (e.g. fileblob behind a custom name) still use gocloudSupportsRange.
func goCloudURLSchemeAssumesRangeSupport(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "s3", "gs", "azblob":
		return true
	default:
		return false
	}
}

// gocloudSupportsRange probes the bucket with a single-byte range read; success = supported.
func (c *CacheExtractorCommand) gocloudSupportsRange(ctx context.Context, b *blob.Bucket, objectName string) bool {
	rr, err := b.NewRangeReader(ctx, objectName, 0, 1, nil)
	if err != nil {
		return false
	}
	_ = rr.Close()
	return true
}

func (c *CacheExtractorCommand) gocloudParallelRangeSupported(ctx context.Context, scheme string, b *blob.Bucket, objectName string) bool {
	if goCloudURLSchemeAssumesRangeSupport(scheme) {
		return true
	}
	return c.gocloudSupportsRange(ctx, b, objectName)
}

func (c *CacheExtractorCommand) handlePresignedURL() error {
	if c.presignedParallelDownloadEligible() {
		done, err := c.tryPresignedParallelDownload()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	return c.downloadPresignedSequential()
}

func (c *CacheExtractorCommand) presignedParallelDownloadEligible() bool {
	logger := logrus.WithField("name", featureflags.UseParallelCacheTransfer)
	return featureflags.IsOn(logger, os.Getenv(featureflags.UseParallelCacheTransfer)) && c.Concurrency > 1
}

// tryPresignedParallelDownload uses a single-byte Range GET (not HEAD: presigned S3 URLs are typically
// signed for GET only). A 206 response yields Content-Length/Content-Range for parallel chunk GETs.
// It returns done=true when the download path finished (including up-to-date short-circuit or parallel
// download); err propagates parallel download failures. done=false, err=nil means fall back to a full GET.
func (c *CacheExtractorCommand) tryPresignedParallelDownload() (done bool, err error) {
	req, reqErr := http.NewRequest(http.MethodGet, c.URL, nil)
	if reqErr != nil {
		return false, nil
	}
	req.Header.Set("Range", "bytes=0-0")

	resp, doErr := c.getClient().Do(req)
	if doErr != nil || resp == nil {
		return false, nil
	}

	if resp.StatusCode != http.StatusPartialContent {
		if resp.StatusCode == http.StatusOK {
			logrus.Infoln("Presigned URL did not honor Range request, using sequential download")
		}
		_ = resp.Body.Close()
		return false, nil
	}

	contentLength, ok := transfer.ParseContentRangeTotal(resp.Header.Get("Content-Range"))
	if !ok {
		_ = resp.Body.Close()
		return false, nil
	}

	date, _ := time.Parse(http.TimeFormat, resp.Header.Get("Last-Modified"))
	if isLocalCacheFileUpToDate(c.File, date) {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, transfer.RangeProbeBodyMaxDiscard))
		_ = resp.Body.Close()
		logrus.Infoln(filepath.Base(c.File), "is up to date")
		return true, nil
	}

	chunkSize := c.effectiveParallelChunkSize()
	if contentLength <= int64(chunkSize) {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, transfer.RangeProbeBodyMaxDiscard))
		_ = resp.Body.Close()
		return false, nil
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, transfer.RangeProbeBodyMaxDiscard))
	_ = resp.Body.Close()

	cleanedURL := url_helpers.CleanURL(c.URL)
	err = c.downloadParallel(contentLength, date, resp.Header.Get("ETag"), cleanedURL, headersToCacheMetadata(resp.Header), c.presignedRangeFetchChunk())
	return true, err
}

func (c *CacheExtractorCommand) downloadPresignedSequential() error {
	resp, err := c.getCache()
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	upToDate, date := checkIfUpToDate(c.File, resp)
	if upToDate {
		logrus.Infoln(filepath.Base(c.File), "is up to date")
		return nil
	}

	etag := resp.Header.Get("ETag")
	cleanedURL := url_helpers.CleanURL(c.URL)
	contentLength := getRemoteCacheSize(resp)

	return c.downloadAndSaveCache(resp.Body, date, etag, cleanedURL, contentLength, headersToCacheMetadata(resp.Header))
}

func (c *CacheExtractorCommand) effectiveParallelChunkSize() int {
	if c.ChunkSize <= 0 {
		return defaultCacheChunkSize
	}
	return c.ChunkSize
}

func (c *CacheExtractorCommand) presignedRangeFetchChunk() func(offset, length int64) (io.ReadCloser, error) {
	return func(offset, length int64) (io.ReadCloser, error) {
		req, err := http.NewRequest(http.MethodGet, c.URL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
		resp, err := c.getClient().Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("range request failed: %s", resp.Status)
		}
		return resp.Body, nil
	}
}

//nolint:gocognit // setup and parallel vs sequential branches
func (c *CacheExtractorCommand) handleGoCloudURL() error {
	if c.mux == nil {
		c.mux = blob.DefaultURLMux()
	}

	ctx, cancelWrite := context.WithCancel(context.Background())
	defer cancelWrite()

	u, err := url.Parse(c.GoCloudURL)
	if err != nil {
		return err
	}

	err = loadEnvFile(c.EnvFile)
	if err != nil {
		return err
	}

	objectName := strings.TrimLeft(u.Path, "/")
	if objectName == "" {
		return fmt.Errorf("no object name provided")
	}

	b, err := c.mux.OpenBucket(ctx, c.GoCloudURL)
	if err != nil {
		return err
	}
	defer b.Close()

	attrs, err := b.Attributes(ctx, objectName)
	if err != nil {
		// Ignore 404 errors
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil
		}
		// GoCloud returns the Unknown code at the moment when Forbidden is returned until
		// https://github.com/google/go-cloud/pull/3663 is merged.
		if u.Scheme == "s3" && strings.Contains(err.Error(), "StatusCode: 403") {
			return fmt.Errorf("%w: This 403 is expected if the file doesn't exist. See the behavior of HeadObject without s3::ListBucket permissions (https://docs.aws.amazon.com/AmazonS3/latest/API/API_HeadObject.html).", err)
		}
		return err
	}

	if isLocalCacheFileUpToDate(c.File, attrs.ModTime) {
		logrus.Infoln(filepath.Base(c.File), "is up to date")
		return nil
	}

	cleanedURL := url_helpers.CleanURL(c.GoCloudURL)

	// Use parallel range reads when FF_USE_PARALLEL_CACHE_TRANSFER is enabled, Concurrency > 1, and backend supports range.
	logger := logrus.WithField("name", featureflags.UseParallelCacheTransfer)
	if featureflags.IsOn(logger, os.Getenv(featureflags.UseParallelCacheTransfer)) && c.Concurrency > 1 && attrs.Size > 0 { //nolint:nestif
		if c.gocloudParallelRangeSupported(ctx, u.Scheme, b, objectName) {
			if attrs.Size > int64(c.effectiveParallelChunkSize()) {
				fetchChunk := func(offset, length int64) (io.ReadCloser, error) {
					return b.NewRangeReader(ctx, objectName, offset, length, nil)
				}
				return c.downloadParallel(attrs.Size, attrs.ModTime, attrs.ETag, cleanedURL, attrs.Metadata, fetchChunk)
			}
		} else {
			logrus.Infoln("GoCloud backend does not support range reads, using sequential download")
		}
	}

	reader, err := b.NewReader(ctx, objectName, nil)
	if err != nil {
		return err
	}
	defer reader.Close()

	return c.downloadAndSaveCache(reader, attrs.ModTime, attrs.ETag, cleanedURL, attrs.Size, attrs.Metadata)
}

// downloadParallel writes content via concurrent range fetches using WriteAt at chunk offsets (bounded memory); the meter counts bytes via WriteAt. fetchChunk returns a reader for the given byte range; caller closes it.
func (c *CacheExtractorCommand) downloadParallel(contentLength int64, modTime time.Time, etag, cleanedURL string, metadata map[string]string, fetchChunk func(offset, length int64) (io.ReadCloser, error)) error { //nolint:gocognit
	file, err := os.CreateTemp(filepath.Dir(c.File), "cache")
	if err != nil {
		return err
	}
	tmpName := file.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	name := strings.TrimSuffix(filepath.Base(c.File), filepath.Ext(c.File))
	if etag != "" {
		logrus.WithField(logFieldHTTPETag, etag).Infoln("Downloading", name, "from", cleanedURL, "(parallel)")
	} else {
		logrus.Infoln("Downloading", name, "from", cleanedURL, "(parallel)")
	}

	writer := meter.NewWriter(
		file,
		c.TransferMeterFrequency,
		meter.LabelledRateFormat(os.Stdout, "Downloading cache", contentLength),
	)
	// writer.Close() closes the underlying file; we must not call file.Close() and we close writer only once (on each exit path below)

	chunkSize := int64(c.effectiveParallelChunkSize())
	concurrency := c.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	destAt, ok := writer.(io.WriterAt)
	if !ok {
		_ = writer.Close()
		return fmt.Errorf("parallel cache download requires destination that implements io.WriterAt")
	}

	err = transfer.ParallelRangeDownload(contentLength, chunkSize, concurrency, destAt, fetchChunk)
	if err != nil {
		_ = writer.Close()
		return retryableErr{err: err}
	}

	if err := writer.Close(); err != nil {
		return err
	}
	// file is closed by writer.Close(); do not call file.Close()
	if err := os.Chtimes(tmpName, time.Now(), modTime); err != nil {
		return err
	}
	if err := os.Rename(tmpName, c.File); err != nil {
		return fmt.Errorf("renaming: %w", err)
	}
	return writeCacheMetadataFile(c.File, metadata)
}

func (c *CacheExtractorCommand) downloadAndSaveCache(reader io.Reader, date time.Time, etag, cleanedURL string, contentLength int64, metadata map[string]string) error {
	file, err := os.CreateTemp(filepath.Dir(c.File), "cache")
	if err != nil {
		return err
	}
	tmpName := file.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	// For legacy purposes, caches written to disk use the extension `.zip`
	// even when a different compression format is used. To avoid confusion,
	// we avoid the extension name in logs.
	name := strings.TrimSuffix(filepath.Base(c.File), filepath.Ext(c.File))

	if etag != "" {
		logrus.WithField(logFieldHTTPETag, etag).Infoln("Downloading", name, "from", cleanedURL)
	} else {
		logrus.Infoln("Downloading", name, "from", cleanedURL)
	}

	writer := meter.NewWriter(
		file,
		c.TransferMeterFrequency,
		meter.LabelledRateFormat(os.Stdout, "Downloading cache", contentLength),
	)
	// writer.Close() closes the underlying file; close writer only once per exit path (same as downloadParallel).

	buf := make([]byte, c.TransferBufferSize)
	_, err = io.CopyBuffer(writer, reader, buf)
	if err != nil {
		_ = writer.Close()
		return retryableErr{err: err}
	}

	err = os.Chtimes(tmpName, time.Now(), date)
	if err != nil {
		_ = writer.Close()
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, c.File); err != nil {
		return fmt.Errorf("renaming: %w", err)
	}

	return writeCacheMetadataFile(c.File, metadata)
}

func (c *CacheExtractorCommand) Execute(cliContext *cli.Context) {
	log.SetRunnerFormatter()

	c.normalizeExtractorArgs()
	if err := validateCacheTransferTuning(c.TransferBufferSize, c.ChunkSize, c.Concurrency); err != nil {
		logrus.Fatalln(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		logrus.Fatalln("Unable to get working directory")
	}

	if c.File == "" {
		warningln("Missing cache file")
	}

	if c.URL != "" || c.GoCloudURL != "" {
		err := c.doRetry(c.download)
		if err != nil {
			warningln(err)
		}
	} else {
		logrus.Infoln(
			"No URL provided, cache will not be downloaded from shared cache server. " +
				"Instead a local version of cache will be extracted.")
	}

	f, size, format, err := openArchive(c.File)
	if os.IsNotExist(err) {
		warningln("Cache file does not exist")
	}
	if err != nil {
		logrus.Fatalln(err)
	}
	defer f.Close()

	extractor, err := archive.NewExtractor(format, f, size, wd)
	if err != nil {
		logrus.Fatalln(err)
	}

	err = extractor.Extract(context.Background())
	if err != nil {
		logrus.Fatalln(err)
	}
}

func warningln(args interface{}) {
	logrus.Warningln(args)
	logrus.Exit(1)
}
