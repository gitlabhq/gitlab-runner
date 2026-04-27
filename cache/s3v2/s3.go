package s3v2

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const DEFAULT_AWS_S3_ENDPOINT = "https://s3.amazonaws.com"
const fallbackBucketLocation = "us-east-1"

const defaultAssumeRoleMaxConcurrency = 5

var assumeRoleInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "gitlab_runner_cache_s3_assume_role_requests_in_flight",
	Help: "Number of AssumeRole requests to AWS STS in progress.",
})

var assumeRoleWaitDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name:    "gitlab_runner_cache_s3_assume_role_wait_seconds",
	Help:    "Wait time to acquire a concurrency slot before an AssumeRole request.",
	Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
})

var assumeRoleCallDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name:    "gitlab_runner_cache_s3_assume_role_duration_seconds",
	Help:    "Duration of AssumeRole API calls to AWS STS.",
	Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
})

var assumeRoleCredCacheHits = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "gitlab_runner_cache_s3_assume_role_cache_hits_total",
	Help: "Number of AssumeRole credential cache hits.",
})

var assumeRoleCredCacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "gitlab_runner_cache_s3_assume_role_cache_misses_total",
	Help: "Number of AssumeRole credential cache misses (This is also a count of the STS calls for cache credentials that were made).",
})

var assumeRoleCredCacheEntries = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
	Name: "gitlab_runner_cache_s3_assume_role_cached_credentials",
	Help: "Current number of AssumeRole credentials held in the LRU cache.",
}, func() float64 { return float64(assumeRoleCredCache.Len()) })

var assumeRoleFailures = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "gitlab_runner_cache_s3_assume_role_failures_total",
	Help: "Number of AssumeRole requests which failed.",
})

// assumeRoleCredCacheSize is the maximum number of AssumeRole credentials held
// in the cache. Each entry is a small map of four env-var strings (~200 B).
// 1 000 entries ≈ 200 KB — sufficient for instance runners serving hundreds
// of projects with multiple cache keys each.
const assumeRoleCredCacheSize = 1000

// assumeRoleCredCacheTTL is the LRU eviction TTL. It matches the maximum
// AssumeRole session duration (1 hour), so the LRU's built-in background
// sweep (runs every TTL/100 ≈ 36 s) cleans up entries that were never
// accessed again after their credential expired.
const assumeRoleCredCacheTTL = time.Hour

// assumeRoleCredCache caches AssumeRole credentials keyed by
// (roleARN, bucketName, objectName, upload). The objectName is deterministic
// (runner/<token>/project/<id>/<cacheKey>), so concurrent jobs sharing the
// same cache key reuse the same credentials without extra STS calls.
//
// The expirable.LRU provides two independent eviction mechanisms:
//   - LRU cap: evicts the least-recently-used entry when the cache is full.
//   - TTL: evicts entries 1 hour after insertion via a background goroutine.
//
// A per-entry expiresAt field is still checked on read so that credentials
// with less remaining validity than required are never returned.
var assumeRoleCredCache = expirable.NewLRU[string, cachedCredential](
	assumeRoleCredCacheSize, nil, assumeRoleCredCacheTTL,
)

type cachedCredential struct {
	creds     map[string]string
	expiresAt time.Time
}

// assumeRoleCacheKey returns a cache key for a set of AssumeRole parameters.
func assumeRoleCacheKey(roleARN, bucketName, objectName string, upload bool) string {
	uploadStr := "0"
	if upload {
		uploadStr = "1"
	}
	return roleARN + "\x00" + bucketName + "\x00" + objectName + "\x00" + uploadStr
}

// FlushCredentialCache evicts all cached AssumeRole credentials, forcing the
// next call for each key to issue a fresh STS request. Use this when a
// credential is known to be compromised or after a configuration change.
func FlushCredentialCache() {
	assumeRoleCredCache.Purge()
}

type s3Presigner interface {
	PresignURL(
		ctx context.Context,
		method string,
		bucketName string,
		objectName string,
		metadata map[string]string,
		expires time.Duration,
	) (cache.PresignedURL, error)
	FetchCredentialsForRole(ctx context.Context, roleARN, bucketName, objectName string, upload bool, timeout time.Duration) (map[string]string, error)
	ServerSideEncryptionType() string
}

type s3Client struct {
	s3Config         *cacheconfig.CacheS3Config
	awsConfig        *aws.Config
	client           *s3.Client
	presignClient    *s3.PresignClient
	stsEndpoint      string
	assumeRoleSem    chan struct{}
	disableCredCache bool
}

type s3ClientOption func(*s3Client)

func withSTSEndpoint(endpoint string) s3ClientOption {
	return func(c *s3Client) {
		c.stsEndpoint = endpoint
	}
}

func withAssumeRoleSem(sem chan struct{}) s3ClientOption {
	return func(c *s3Client) {
		c.assumeRoleSem = sem
	}
}

func (c *s3Client) PresignURL(ctx context.Context,
	method string,
	bucketName string,
	objectName string,
	metadata map[string]string,
	expires time.Duration) (cache.PresignedURL, error) {
	var presignedReq *v4.PresignedHTTPRequest
	var err error

	switch method {
	case http.MethodGet:
		getObjectInput := &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectName),
		}
		presignedReq, err = c.presignClient.PresignGetObject(ctx, getObjectInput, s3.WithPresignExpires(expires))
	case http.MethodPut:
		putObjectInput := &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectName),
		}
		if len(metadata) > 0 {
			putObjectInput.Metadata = metadata
		}
		switch c.s3Config.EncryptionType() {
		case cacheconfig.S3EncryptionTypeAes256:
			putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAes256
		case cacheconfig.S3EncryptionTypeKms:
			putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
			putObjectInput.SSEKMSKeyId = aws.String(c.s3Config.ServerSideEncryptionKeyID)
		case cacheconfig.S3EncryptionTypeDsseKms:
			putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAwsKmsDsse
			putObjectInput.SSEKMSKeyId = aws.String(c.s3Config.ServerSideEncryptionKeyID)
		}
		presignedReq, err = c.presignClient.PresignPutObject(ctx, putObjectInput, s3.WithPresignExpires(expires))
	default:
		return cache.PresignedURL{}, fmt.Errorf("unsupported method: %s", method)
	}

	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")
		return cache.PresignedURL{}, err
	}

	u, err := url.Parse(presignedReq.URL)
	if err != nil {
		logrus.WithError(err).WithField("url", presignedReq.URL).Errorf("error parsing S3 URL")
		return cache.PresignedURL{}, err
	}

	return cache.PresignedURL{URL: u, Headers: presignedReq.SignedHeader}, nil
}

func (c *s3Client) generateSessionPolicy(bucketName, objectName string, upload bool) string {
	action := "s3:GetObject"
	if upload {
		action = "s3:PutObject"
	}

	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html
	s3Partition := "aws"
	// https://docs.aws.amazon.com/govcloud-us/latest/UserGuide/using-govcloud-arns.html
	switch {
	case strings.HasPrefix(c.awsConfig.Region, "us-gov-"):
		s3Partition = "aws-us-gov"
	case strings.HasPrefix(c.awsConfig.Region, "cn-"):
		s3Partition = "aws-cn"
	}

	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": ["%s"],
				"Resource": "arn:%s:s3:::%s/%s"
			}`, action, s3Partition, bucketName, objectName)

	if c.s3Config.EncryptionType() == cacheconfig.S3EncryptionTypeKms || c.s3Config.EncryptionType() == cacheconfig.S3EncryptionTypeDsseKms {
		// Permissions needed for multipart upload: https://repost.aws/knowledge-center/s3-large-file-encryption-kms-key
		policy += fmt.Sprintf(`,
			{
				"Effect": "Allow",
				"Action": [
					"kms:Decrypt",
					"kms:GenerateDataKey"
				],
				"Resource": "%s"
			}`, c.s3Config.ServerSideEncryptionKeyID)
	}

	policy += `
	]
}`

	return policy
}

// cachedCreds returns credentials from the cache if they have at least
// minValidity of remaining lifetime. Returns (nil, false) on a cache miss,
// a disabled cache, or insufficient remaining validity.
func (c *s3Client) cachedCreds(credKey string, minValidity time.Duration) (map[string]string, bool) {
	if c.disableCredCache {
		return nil, false
	}
	cached, ok := assumeRoleCredCache.Get(credKey)
	if !ok || time.Until(cached.expiresAt) < minValidity {
		return nil, false
	}
	assumeRoleCredCacheHits.Inc()
	return cached.creds, true
}

// acquireAssumeRoleSem acquires a slot in the concurrency semaphore and
// returns a release function. If no semaphore is configured the release
// function is a no-op. Returns an error if ctx is cancelled while waiting.
func (c *s3Client) acquireAssumeRoleSem(ctx context.Context) (func(), error) {
	if c.assumeRoleSem == nil {
		return func() {}, nil
	}
	waitStart := time.Now()
	select {
	case c.assumeRoleSem <- struct{}{}:
		assumeRoleWaitDuration.Observe(time.Since(waitStart).Seconds())
		assumeRoleInFlight.Inc()
		return func() {
			<-c.assumeRoleSem
			assumeRoleInFlight.Dec()
		}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled waiting for AssumeRole semaphore: %w", ctx.Err())
	}
}

func (c *s3Client) FetchCredentialsForRole(ctx context.Context, roleARN, bucketName, objectName string, upload bool, timeout time.Duration) (map[string]string, error) {
	// minValidity is the minimum remaining lifetime a cached credential must
	// have to be considered usable. We want credentials to remain valid for
	// the entire transfer (at least `timeout`), but cap at 55 minutes so
	// that cache hits are always possible within the 1-hour session lifetime,
	// regardless of how large `timeout` is configured.
	minValidity := min(max(timeout, time.Minute), 55*time.Minute)
	credKey := assumeRoleCacheKey(roleARN, bucketName, objectName, upload)

	// Fast path: return cached credentials without touching the semaphore.
	if creds, ok := c.cachedCreds(credKey, minValidity); ok {
		return creds, nil
	}

	sessionPolicy := c.generateSessionPolicy(bucketName, objectName, upload)

	stsClient := sts.NewFromConfig(*c.awsConfig, func(o *sts.Options) {
		if c.stsEndpoint != "" {
			o.BaseEndpoint = aws.String(c.stsEndpoint)
		}
	})
	uuid, err := helpers.GenerateRandomUUID(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random UUID: %w", err)
	}
	sessionName := fmt.Sprintf("gitlab-runner-cache-upload-%s", uuid)

	// Request the maximum allowed session duration. Credentials are cached
	// and reused across jobs, so a longer session duration means more cache
	// hits. According to https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_manage-assume.html#id_roles_use_view-role-max-session,
	// session durations must be between 15 minutes and 12 hours; when role
	// chaining is in use, AWS limits this to 1 hour.
	const duration = 1 * time.Hour

	release, err := c.acquireAssumeRoleSem(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	// Double-check cache after acquiring the semaphore slot. A concurrent
	// goroutine may have fetched and cached credentials for the same key
	// while we were waiting.
	if creds, ok := c.cachedCreds(credKey, minValidity); ok {
		return creds, nil
	}

	assumeRoleCredCacheMisses.Inc()
	startTime := time.Now()
	roleCredentials, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleARN),
		RoleSessionName: aws.String(sessionName),
		Policy:          aws.String(sessionPolicy), // Limit the role's access
		DurationSeconds: aws.Int32(int32(duration.Seconds())),
	})
	elapsed := time.Since(startTime).Seconds()
	assumeRoleCallDuration.Observe(elapsed)

	if err != nil {
		assumeRoleFailures.Inc()
		logrus.WithError(err).WithFields(logrus.Fields{
			"role_arn":   roleARN,
			"duration_s": elapsed,
		}).Error("Failed to assume role for cache credentials")
		return nil, fmt.Errorf("failed to assume role (took %.2fs): %w", elapsed, err)
	}
	// AssumeRole should always return credentials if successful, but
	// just in case it doesn't let's check this.
	if roleCredentials.Credentials == nil {
		assumeRoleFailures.Inc()
		logrus.WithFields(logrus.Fields{
			"role_arn":   roleARN,
			"duration_s": elapsed,
		}).Error("AssumeRole succeeded but returned no credentials")
		return nil, fmt.Errorf("failed to retrieve credentials (took %.2fs): %w", elapsed, err)
	}

	logrus.WithFields(logrus.Fields{
		"role_arn":   roleARN,
		"duration_s": elapsed,
	}).Debug("Successfully assumed role for cache credentials")

	creds := map[string]string{
		"AWS_ACCESS_KEY_ID":     *roleCredentials.Credentials.AccessKeyId,
		"AWS_SECRET_ACCESS_KEY": *roleCredentials.Credentials.SecretAccessKey,
		"AWS_SESSION_TOKEN":     *roleCredentials.Credentials.SessionToken,
		"AWS_PROFILE":           "", // Ignore user-defined values
	}

	// Cache only when the response includes an expiration. This is always
	// the case for AssumeRole, but we guard defensively to avoid storing
	// credentials that we cannot expire correctly.
	if !c.disableCredCache && roleCredentials.Credentials.Expiration != nil {
		assumeRoleCredCache.Add(credKey, cachedCredential{
			creds:     creds,
			expiresAt: *roleCredentials.Credentials.Expiration,
		})
	}

	return creds, nil
}

func (c *s3Client) ServerSideEncryptionType() string {
	return s3EncryptionType(c.s3Config.EncryptionType())
}

func s3EncryptionType(encryptionType cacheconfig.S3EncryptionType) string {
	switch encryptionType {
	case cacheconfig.S3EncryptionTypeAes256:
		return string(types.ServerSideEncryptionAes256)
	case cacheconfig.S3EncryptionTypeKms:
		return string(types.ServerSideEncryptionAwsKms)
	case cacheconfig.S3EncryptionTypeDsseKms:
		return string(types.ServerSideEncryptionAwsKmsDsse)
	default:
		return ""
	}
}

func newRawS3Client(s3Config *cacheconfig.CacheS3Config) (*aws.Config, *s3.Client, error) {
	var cfg aws.Config
	var err error
	options := make([]func(*config.LoadOptions) error, 0)

	endpoint := s3Config.GetEndpoint()

	switch s3Config.AuthType() {
	case cacheconfig.S3AuthTypeIAM:
		break
	case cacheconfig.S3AuthTypeAccessKey:
		options = append(options,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3Config.AccessKey, s3Config.SecretKey, s3Config.SessionToken)),
		)
	}

	bucketLocation := s3Config.BucketLocation
	if bucketLocation == "" {
		bucketLocation = detectBucketLocation(s3Config, options...)
	}

	options = append(options, config.WithRegion(bucketLocation))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err = config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		// To preserve backwards compatibility, configs that set ServerAddress to
		// "s3.amazonaws.com" don't need a custom endpoint since that is the default
		// S3 address.
		//
		// The AWS SDK doesn't allow you to generate a pre-signed URL with a custom endpoint
		// and DualStack or Accelerate options set.
		if endpoint != "" && endpoint != DEFAULT_AWS_S3_ENDPOINT {
			o.BaseEndpoint = aws.String(endpoint)
		} else {
			o.UseDualstack = s3Config.DualStackEnabled() // nolint:staticcheck
			o.UseAccelerate = s3Config.Accelerate
		}
		o.UsePathStyle = s3Config.PathStyleEnabled()
	})

	return &cfg, client, nil
}

func detectBucketLocation(s3Config *cacheconfig.CacheS3Config, optFuncs ...func(*config.LoadOptions) error) string {
	// The 30 seconds timeout here is arbritrary
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When s3 is configured with an IAM profile, a default region must be set
	// We therefore set the default region to us-east-1
	configOpts := append(
		[]func(*config.LoadOptions) error{
			config.WithRegion(fallbackBucketLocation),
		},
		optFuncs...,
	)

	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return fallbackBucketLocation
	}

	endpoint := s3Config.GetEndpoint()
	effectiveEndpoint := DEFAULT_AWS_S3_ENDPOINT
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" && endpoint != DEFAULT_AWS_S3_ENDPOINT {
			o.BaseEndpoint = aws.String(endpoint)
			effectiveEndpoint = endpoint
		}
		o.UsePathStyle = s3Config.PathStyleEnabled()
	})
	output, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(s3Config.BucketName),
	})

	logEntry := logrus.WithFields(logrus.Fields{
		"endpoint": effectiveEndpoint,
		"bucket":   s3Config.BucketName,
	})

	if err != nil {
		logEntry.WithError(err).Warning("Failed to detect S3 bucket location, falling back to default region")
		return fallbackBucketLocation
	}

	location := string(output.LocationConstraint)
	switch output.LocationConstraint {
	case "":
		location = fallbackBucketLocation
	case types.BucketLocationConstraintEu:
		location = string(types.BucketLocationConstraintEuWest1)
	}

	logEntry.WithField("location", location).Debug("Successfully detected S3 bucket location")
	return location
}

// clientInit holds a lazily-built s3Client. sync.Once ensures that concurrent
// callers for the same s3Config pointer share a single buildS3Client call.
type clientInit struct {
	once   sync.Once
	client s3Presigner
	err    error
}

// s3ClientCache maps *cacheconfig.CacheS3Config → *clientInit.
var s3ClientCache sync.Map

// buildS3Client constructs a new s3Client without any caching.
func buildS3Client(s3Config *cacheconfig.CacheS3Config, options ...s3ClientOption) (s3Presigner, error) {
	cfg, client, err := newRawS3Client(s3Config)
	if err != nil {
		return nil, err
	}

	presignClient := s3.NewPresignClient(client)

	concurrency := s3Config.AssumeRoleMaxConcurrency
	var assumeRoleSem chan struct{}
	switch {
	case concurrency == 0:
		assumeRoleSem = make(chan struct{}, defaultAssumeRoleMaxConcurrency)
	case concurrency > 0:
		assumeRoleSem = make(chan struct{}, concurrency)
		// concurrency < 0: nil channel, semaphore disabled
	}

	c := &s3Client{
		s3Config:         s3Config,
		awsConfig:        cfg,
		client:           client,
		presignClient:    presignClient,
		assumeRoleSem:    assumeRoleSem,
		disableCredCache: s3Config.DisableAssumeRoleCredentialsCaching,
	}

	for _, opt := range options {
		opt(c)
	}

	return c, nil
}

// newS3Client returns a cached s3Client for the given config when possible.
//
// The s3Config pointer is used as the cache key. Each config load allocates a
// fresh CacheS3Config (TOML unmarshal creates new objects), so pointer identity
// naturally captures both "which runner" and "which load": after a config
// reload the pointer changes and the old entry is never matched again.
//
// Caching is skipped when options are provided (options such as withSTSEndpoint
// mutate the client and must not be shared across callers).
//
// sync.Once inside clientInit ensures that concurrent callers sharing the same
// s3Config pointer issue only one newRawS3Client call (and therefore one IMDS
// request) even during the initial population or after a reload.
var newS3Client = func(s3Config *cacheconfig.CacheS3Config, options ...s3ClientOption) (s3Presigner, error) {
	if len(options) > 0 {
		return buildS3Client(s3Config, options...)
	}

	init := &clientInit{}
	actual, _ := s3ClientCache.LoadOrStore(s3Config, init)
	ci, ok := actual.(*clientInit)
	if !ok {
		return buildS3Client(s3Config)
	}
	ci.once.Do(func() {
		ci.client, ci.err = buildS3Client(s3Config)
		if ci.err != nil {
			s3ClientCache.CompareAndDelete(s3Config, ci)
		}
	})
	return ci.client, ci.err
}
