//go:build !integration

package s3v2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	dto "github.com/prometheus/client_model/go"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sessionPolicy struct {
	Version   string            `json:"Version"`
	Statement []policyStatement `json:"Statement"`
}

type policyStatement struct {
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource string   `json:"Resource"`
}

func setupMockS3Server(t *testing.T) *cacheconfig.CacheS3Config {
	backend := s3mem.New()
	server := gofakes3.New(backend)
	ts := httptest.NewServer(server.Server())
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	url, err := url.Parse(ts.URL)
	require.NoError(t, err)

	s3Config := &cacheconfig.CacheS3Config{
		ServerAddress:  url.Host,
		Insecure:       true,
		BucketLocation: "us-west-1",
		BucketName:     "test-bucket",
		AccessKey:      "test-access-key",
		SecretKey:      "test-secret-key",
	}

	t.Cleanup(func() {
		ts.Close()
	})

	_, client, err := newRawS3Client(s3Config)
	require.NoError(t, err)

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s3Config.BucketName),
	})
	require.NoError(t, err)

	return s3Config
}

func TestS3ClientCaching(t *testing.T) {
	s3Config := &cacheconfig.CacheS3Config{
		AccessKey:      "test-access-key",
		SecretKey:      "test-secret-key",
		BucketName:     "test-bucket",
		BucketLocation: "us-west-2",
	}

	t.Cleanup(func() {
		s3ClientCache.Delete(s3Config)
	})

	c1, err := newS3Client(s3Config)
	require.NoError(t, err)

	// Same pointer returns the same instance.
	c2, err := newS3Client(s3Config)
	require.NoError(t, err)
	assert.Same(t, c1.(*s3Client), c2.(*s3Client))

	// A different pointer (simulating a config reload) returns a new instance.
	reloadedConfig := *s3Config
	t.Cleanup(func() {
		s3ClientCache.Delete(&reloadedConfig)
	})
	c3, err := newS3Client(&reloadedConfig)
	require.NoError(t, err)
	assert.NotSame(t, c1.(*s3Client), c3.(*s3Client))

	// Options bypass the cache entirely.
	c4, err := newS3Client(s3Config, withSTSEndpoint("http://sts.example.com"))
	require.NoError(t, err)
	assert.NotSame(t, c1.(*s3Client), c4.(*s3Client))
}

func TestNewS3ClientOptions(t *testing.T) {
	disableDualStack := false

	tests := map[string]struct {
		s3Config            cacheconfig.CacheS3Config
		expectedStaticCreds bool
		expectedRegion      string
		expectedScheme      string
		usePathStyle        bool
		expectedAccelerate  bool
		expectedDualStack   bool
		expectedEndpoint    string
	}{
		"s3-standard": {
			s3Config: cacheconfig.CacheS3Config{
				AccessKey:      "test-access-key",
				SecretKey:      "test-secret-key",
				ServerAddress:  "s3.amazonaws.com",
				BucketName:     "test-bucket",
				BucketLocation: "us-west-2",
			},
			expectedStaticCreds: true,
			expectedRegion:      "us-west-2",
			expectedScheme:      "https",
			expectedEndpoint:    "",
			expectedDualStack:   true,
		},
		"s3-standard-with-session-token": {
			s3Config: cacheconfig.CacheS3Config{
				AccessKey:      "test-access-key",
				SecretKey:      "test-secret-key",
				SessionToken:   "test-session-token",
				ServerAddress:  "s3.amazonaws.com",
				BucketName:     "test-bucket",
				BucketLocation: "us-west-2",
			},
			expectedStaticCreds: true,
			expectedRegion:      "us-west-2",
			expectedScheme:      "https",
			expectedEndpoint:    "",
			expectedDualStack:   true,
		},
		"s3-standard-dual-stack": {
			s3Config: cacheconfig.CacheS3Config{
				BucketName:     "test-bucket",
				BucketLocation: "us-west-2",
				DualStack:      &disableDualStack,
			},
			expectedDualStack: false,
			expectedRegion:    "us-west-2",
			expectedScheme:    "https",
			expectedEndpoint:  "",
		},
		"s3-default-address-set": {
			s3Config: cacheconfig.CacheS3Config{
				BucketName:     "test-bucket",
				BucketLocation: "us-west-2",
				ServerAddress:  "s3.amazonaws.com",
			},
			expectedDualStack: true,
			expectedRegion:    "us-west-2",
			expectedScheme:    "https",
			expectedEndpoint:  "",
		},
		"s3-iam-profile": {
			s3Config: cacheconfig.CacheS3Config{
				BucketName:     "test-bucket",
				BucketLocation: "us-west-2",
			},
			expectedRegion:    "us-west-2",
			expectedScheme:    "https",
			expectedEndpoint:  "",
			expectedDualStack: true,
		},
		"s3-accelerate": {
			s3Config: cacheconfig.CacheS3Config{
				BucketName:     "test-bucket",
				BucketLocation: "us-east-1",
				Accelerate:     true,
			},
			expectedRegion:     "us-east-1",
			expectedScheme:     "https",
			expectedAccelerate: true,
			expectedDualStack:  true,
		},
		"s3-accelerate-custom-endpoint": {
			s3Config: cacheconfig.CacheS3Config{
				ServerAddress:  "s3-accelerate.amazonaws.com",
				BucketName:     "test-bucket",
				BucketLocation: "us-east-1",
			},
			expectedRegion:     "us-east-1",
			expectedScheme:     "https",
			expectedEndpoint:   "https://s3-accelerate.amazonaws.com",
			expectedAccelerate: true,
			expectedDualStack:  false,
		},
		"s3-custom-endpoint": {
			s3Config: cacheconfig.CacheS3Config{
				ServerAddress:  "localhost:9000",
				BucketName:     "test-bucket",
				BucketLocation: "us-west-2",
				Insecure:       true,
			},
			expectedRegion:    "us-west-2",
			expectedScheme:    "http",
			usePathStyle:      true, // Not virtual-host compatible
			expectedEndpoint:  "http://localhost:9000",
			expectedDualStack: false,
		},
		"s3-dual-stack": {
			s3Config: cacheconfig.CacheS3Config{
				BucketName:     "test-bucket",
				BucketLocation: "us-east-1",
			},
			expectedRegion:    "us-east-1",
			expectedScheme:    "https",
			usePathStyle:      false,
			expectedDualStack: true,
		},
		"s3-dual-stack-and-accelerate": {
			s3Config: cacheconfig.CacheS3Config{
				BucketName:     "test-bucket",
				BucketLocation: "us-east-1",
				Accelerate:     true,
			},
			expectedRegion:    "us-east-1",
			expectedScheme:    "https",
			usePathStyle:      false,
			expectedDualStack: true,
		},
		"s3-dual-stack-and-endpoint": {
			s3Config: cacheconfig.CacheS3Config{
				ServerAddress:  "localhost:9000",
				BucketName:     "test-bucket",
				BucketLocation: "us-east-1",
			},
			expectedRegion:    "us-east-1",
			expectedScheme:    "https",
			usePathStyle:      true,
			expectedEndpoint:  "https://localhost:9000",
			expectedDualStack: false,
		},
		"s3-no-region": {
			s3Config: cacheconfig.CacheS3Config{
				ServerAddress: "localhost:9000",
				BucketName:    "test-bucket",
			},
			expectedRegion:    "us-east-1",
			expectedScheme:    "https",
			usePathStyle:      true,
			expectedEndpoint:  "https://localhost:9000",
			expectedDualStack: false,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			client, err := newS3Client(&tt.s3Config)
			require.NoError(t, err)

			s3Client := client.(*s3Client).client

			if tt.expectedStaticCreds {
				credsProvider := s3Client.Options().Credentials

				creds, err := credsProvider.Retrieve(t.Context())
				require.NoError(t, err)
				require.Equal(t, tt.s3Config.AccessKey, creds.AccessKeyID)
				require.Equal(t, tt.s3Config.SecretKey, creds.SecretAccessKey)
				require.Equal(t, tt.s3Config.SessionToken, creds.SessionToken)
			}

			clientOptions := s3Client.Options()
			require.Equal(t, tt.expectedRegion, clientOptions.Region)
			require.Equal(t, tt.s3Config.Accelerate, clientOptions.UseAccelerate)
			require.Equal(t, tt.expectedDualStack, clientOptions.UseDualstack) // nolint:staticcheck
			require.Equal(t, tt.usePathStyle, clientOptions.UsePathStyle)

			if tt.expectedEndpoint == "" {
				require.Nil(t, clientOptions.BaseEndpoint)
			} else {
				require.Equal(t, tt.expectedEndpoint, *clientOptions.BaseEndpoint)
			}
		})
	}
}

func TestS3Client_PresignURL(t *testing.T) {
	s3Config := setupMockS3Server(t)

	tests := map[string]struct {
		encryptionType     string
		encryptionKeyID    string
		accessKey          string
		secretKey          string
		expectedEncryption string
		expectedKMSKeyID   string
	}{
		"no-encryption-with-credentials": {
			encryptionType:     "",
			accessKey:          "test-access-key",
			secretKey:          "test-secret-key",
			expectedEncryption: "",
			expectedKMSKeyID:   "",
		},
		"s3-encryption-with-credentials": {
			encryptionType:     "S3",
			accessKey:          "test-access-key",
			secretKey:          "test-secret-key",
			expectedEncryption: "AES256",
			expectedKMSKeyID:   "",
		},
		"kms-encryption-with-credentials": {
			encryptionType:     "KMS",
			encryptionKeyID:    "alias/my-key",
			accessKey:          "test-access-key",
			secretKey:          "test-secret-key",
			expectedEncryption: "aws:kms",
			expectedKMSKeyID:   "alias/my-key",
		},
		"kms-dsse-encryption-with-credentials": {
			encryptionType:     "DSSE-KMS",
			encryptionKeyID:    "alias/my-key",
			accessKey:          "test-access-key",
			secretKey:          "test-secret-key",
			expectedEncryption: "aws:kms:dsse",
			expectedKMSKeyID:   "alias/my-key",
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			s3Config.ServerSideEncryption = tt.encryptionType
			s3Config.ServerSideEncryptionKeyID = tt.encryptionKeyID
			s3Config.AccessKey = tt.accessKey
			s3Config.SecretKey = tt.secretKey

			s3Client, err := newS3Client(s3Config)
			require.NoError(t, err)

			// Presign a PUT request to upload an object
			objectName := "test-object"
			url, err := s3Client.PresignURL(t.Context(), http.MethodPut, s3Config.BucketName, objectName, nil, 5*time.Minute)
			require.NoError(t, err)

			// Verify encryption headers
			if tt.expectedEncryption != "" {
				assert.Equal(t, tt.expectedEncryption, url.Headers.Get("x-amz-server-side-encryption"))
			}

			if tt.expectedKMSKeyID != "" {
				assert.Equal(t, tt.expectedKMSKeyID, url.Headers.Get("x-amz-server-side-encryption-aws-kms-key-id"))
			}

			// Use the presigned URL to upload an object
			content := []byte("Hello, world!")
			req, err := http.NewRequest(http.MethodPut, url.URL.String(), bytes.NewReader(content))
			require.NoError(t, err)

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			resp.Body.Close()

			// Presign a GET request to download the object
			url, err = s3Client.PresignURL(t.Context(), http.MethodGet, s3Config.BucketName, objectName, nil, 5*time.Minute)
			require.NoError(t, err)

			req, err = http.NewRequest(http.MethodGet, url.URL.String(), bytes.NewReader(content))
			require.NoError(t, err)

			resp, err = client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, content, body)
		})
	}
}

func newMockSTSHandler(expectedKms bool, expectedDurationSecs int, s3Partition string) http.Handler {
	roleARN := "arn:aws:iam::123456789012:role/TestRole"
	expectedStatements := 1
	if expectedKms {
		expectedStatements = 2
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sts" {
			http.NotFound(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		queryValues, err := url.ParseQuery(string(body))
		if err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		if queryValues.Get("Action") != "AssumeRole" {
			http.Error(w, "Invalid Action parameter", http.StatusBadRequest)
			return
		}

		if queryValues.Get("RoleArn") == "" {
			http.Error(w, "Missing RoleArn parameter", http.StatusBadRequest)
			return
		}

		if queryValues.Get("RoleArn") != roleARN {
			http.Error(w, "Invalid RoleArn parameter", http.StatusUnauthorized)
			return
		}

		if queryValues.Get("DurationSeconds") != fmt.Sprintf("%d", expectedDurationSecs) {
			http.Error(w, "Invalid DurationSeconds parameter", http.StatusUnauthorized)
			return
		}

		if queryValues.Get("RoleSessionName") == "" {
			http.Error(w, "Missing RoleSessionName parameter", http.StatusBadRequest)
			return
		}

		policy := queryValues.Get("Policy")
		if policy == "" {
			http.Error(w, "Missing Policy parameter", http.StatusBadRequest)
			return
		}

		var policyJSON sessionPolicy
		err = json.Unmarshal([]byte(policy), &policyJSON)
		if err != nil {
			http.Error(w, "Invalid Policy JSON", http.StatusBadRequest)
			return
		}

		if policyJSON.Statement == nil || len(policyJSON.Statement) != expectedStatements {
			http.Error(w, fmt.Sprintf("Policy must contain exactly %d Statements", expectedStatements), http.StatusBadRequest)
			return
		}

		statement := policyJSON.Statement[0]
		if statement.Action == nil || len(statement.Action) != 1 {
			http.Error(w, "Statement must contain exactly one Action", http.StatusBadRequest)
			return
		}

		if statement.Action[0] != "s3:PutObject" {
			http.Error(w, "Action should be s3:PutObject", http.StatusBadRequest)
			return
		}

		if expectedKms {
			kmsStatement := policyJSON.Statement[1]
			if kmsStatement.Action == nil || len(kmsStatement.Action) != 2 {
				http.Error(w, "KMS Statement must contain exactly two Actions", http.StatusBadRequest)
				return
			}
			if kmsStatement.Action[0] != "kms:Decrypt" || kmsStatement.Action[1] != "kms:GenerateDataKey" {
				http.Error(w, "KMS Statement Actions should be kms:Decrypt and kms:GenerateDataKey", http.StatusBadRequest)
				return
			}
		}

		if s3Partition == "" {
			s3Partition = "aws"
		}
		if statement.Resource != fmt.Sprintf("arn:%s:s3:::%s/%s", s3Partition, bucketName, objectName) {
			http.Error(w, "Invalid policy statement", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		// See https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
		_, err = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <AccessKeyId>mock-access-key</AccessKeyId>
      <SecretAccessKey>mock-secret-key</SecretAccessKey>
      <SessionToken>mock-session-token</SessionToken>
      <Expiration>` + time.Now().Add(time.Hour).Format(time.RFC3339) + `</Expiration>
    </Credentials>
    <AssumedRoleUser>
      <AssumedRoleId>AROATEST123:TestSession</AssumedRoleId>
      <Arn>arn:aws:sts::123456789012:assumed-role/TestRole/TestSession</Arn>
    </AssumedRoleUser>
  </AssumeRoleResult>
  <ResponseMetadata>
    <RequestId>c6104cbe-af31-11e0-8154-cbc7ccf896c7</RequestId>
  </ResponseMetadata>
</AssumeRoleResponse>`))
		if err != nil {
			w.WriteHeader(http.StatusExpectationFailed)
		}
	})
}

func TestFetchCredentialsForRole(t *testing.T) {
	workingConfig := cacheconfig.Config{
		S3: &cacheconfig.CacheS3Config{
			AccessKey:          "test-access-key",
			SecretKey:          "test-secret-key",
			AuthenticationType: "access-key",
			BucketName:         "test-bucket",
			UploadRoleARN:      "arn:aws:iam::123456789012:role/TestRole",
		},
	}
	mockedCreds := map[string]string{
		"AWS_ACCESS_KEY_ID":     "mock-access-key",
		"AWS_SECRET_ACCESS_KEY": "mock-secret-key",
		"AWS_SESSION_TOKEN":     "mock-session-token",
		"AWS_PROFILE":           "",
	}
	govCloudConfig := cacheconfig.Config{
		S3: &cacheconfig.CacheS3Config{
			AccessKey:          "test-access-key",
			BucketLocation:     "us-gov-west-1",
			SecretKey:          "test-secret-key",
			AuthenticationType: "access-key",
			BucketName:         "test-bucket",
			UploadRoleARN:      "arn:aws:iam::123456789012:role/TestRole",
		},
	}
	chinaConfig := cacheconfig.Config{
		S3: &cacheconfig.CacheS3Config{
			AccessKey:          "test-access-key",
			BucketLocation:     "cn-north-1",
			SecretKey:          "test-secret-key",
			AuthenticationType: "access-key",
			BucketName:         "test-bucket",
			UploadRoleARN:      "arn:aws:iam::123456789012:role/TestRole",
		},
	}

	tests := map[string]struct {
		config           *cacheconfig.Config
		roleARN          string
		expected         map[string]string
		errMsg           string
		expectedKms      bool
		duration         time.Duration
		expectedDuration time.Duration
		s3Partition      string
	}{
		"successful fetch": {
			config:   &workingConfig,
			roleARN:  "arn:aws:iam::123456789012:role/TestRole",
			expected: mockedCreds,
		},
		"successful fetch with GovCloud config": {
			config:      &govCloudConfig,
			roleARN:     "arn:aws:iam::123456789012:role/TestRole",
			expected:    mockedCreds,
			s3Partition: "aws-us-gov",
		},
		"successful fetch with China config": {
			config:      &chinaConfig,
			roleARN:     "arn:aws:iam::123456789012:role/TestRole",
			expected:    mockedCreds,
			s3Partition: "aws-cn",
		},
		"successful fetch with 12-hour timeout downgraded to 1-hour": {
			config:           &workingConfig,
			roleARN:          "arn:aws:iam::123456789012:role/TestRole",
			duration:         12 * time.Hour,
			expected:         mockedCreds,
			expectedDuration: 1 * time.Hour,
		},
		"successful fetch with 10-minute timeout": {
			config:           &workingConfig,
			roleARN:          "arn:aws:iam::123456789012:role/TestRole",
			duration:         10 * time.Minute,
			expected:         mockedCreds,
			expectedDuration: 1 * time.Hour,
		},
		"successful fetch with 13-hour timeout": {
			config:           &workingConfig,
			roleARN:          "arn:aws:iam::123456789012:role/TestRole",
			duration:         13 * time.Hour,
			expected:         mockedCreds,
			expectedDuration: 1 * time.Hour,
		},
		"successful fetch with encryption": {
			config: &cacheconfig.Config{
				S3: &cacheconfig.CacheS3Config{
					AccessKey:                 "test-access-key",
					SecretKey:                 "test-secret-key",
					AuthenticationType:        "access-key",
					BucketName:                "test-bucket",
					UploadRoleARN:             "arn:aws:iam::123456789012:role/TestRole",
					ServerSideEncryption:      "KMS",
					ServerSideEncryptionKeyID: "arn:aws:kms:us-west-2:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab",
				},
			},
			roleARN:     "arn:aws:iam::123456789012:role/TestRole",
			expected:    mockedCreds,
			expectedKms: true,
		},
		"invalid role ARN": {
			config: &cacheconfig.Config{
				S3: &cacheconfig.CacheS3Config{
					AccessKey:          "test-access-key",
					SecretKey:          "test-secret-key",
					AuthenticationType: "access-key",
					BucketName:         bucketName,
					UploadRoleARN:      "arn:aws:iam::123456789012:role/InvalidRole",
				},
			},
			roleARN: "arn:aws:iam::123456789012:role/InvalidRole",
			errMsg:  "failed to assume role",
		},
		"no role ARN": {
			config: &cacheconfig.Config{
				S3: &cacheconfig.CacheS3Config{
					AccessKey:          "test-access-key",
					SecretKey:          "test-secret-key",
					AuthenticationType: "access-key",
					BucketName:         bucketName,
				},
			},
			expected: nil,
			errMsg:   "failed to assume role",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			FlushCredentialCache()
			t.Cleanup(FlushCredentialCache)

			duration := 3600
			if tt.duration > 0 {
				duration = int(tt.expectedDuration.Seconds())
			}
			// Create s3Client and point STS endpoint to it
			mockServer := httptest.NewServer(newMockSTSHandler(tt.expectedKms, duration, tt.s3Partition))
			defer mockServer.Close()

			s3Client, err := newS3Client(tt.config.S3, withSTSEndpoint(mockServer.URL+"/sts"))
			require.NoError(t, err)

			creds, err := s3Client.FetchCredentialsForRole(t.Context(), tt.roleARN, bucketName, objectName, true, tt.duration)

			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, creds)
			}
		})
	}
}

func histogramSampleCount(t *testing.T, h prometheus.Histogram) uint64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, h.Write(&m))
	return m.GetHistogram().GetSampleCount()
}

// TestFetchCredentialsForRole_ConcurrencyLimit verifies that at most 5
// AssumeRole calls are in-flight at any time.
func TestFetchCredentialsForRole_ConcurrencyLimit(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	const semSize = 5
	const numRequests = 8

	testSem := make(chan struct{}, semSize)

	var currentInFlight atomic.Int32
	reached := make(chan struct{}, numRequests)
	release := make(chan struct{})

	successXML := `<?xml version="1.0" encoding="UTF-8"?>
<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <AccessKeyId>mock-access-key</AccessKeyId>
      <SecretAccessKey>mock-secret-key</SecretAccessKey>
      <SessionToken>mock-session-token</SessionToken>
      <Expiration>` + time.Now().Add(time.Hour).Format(time.RFC3339) + `</Expiration>
    </Credentials>
  </AssumeRoleResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</AssumeRoleResponse>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sts" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()

		currentInFlight.Add(1)
		reached <- struct{}{}
		<-release
		currentInFlight.Add(-1)

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(successXML))
	}))
	defer server.Close()

	s3Config := &cacheconfig.CacheS3Config{
		AccessKey:          "test-access-key",
		SecretKey:          "test-secret-key",
		AuthenticationType: "access-key",
		BucketName:         bucketName,
		BucketLocation:     "us-east-1",
	}
	client, err := newS3Client(s3Config, withSTSEndpoint(server.URL+"/sts"), withAssumeRoleSem(testSem))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range numRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.FetchCredentialsForRole(t.Context(), "arn:aws:iam::123456789012:role/TestRole", bucketName, objectName, true, 0)
		}()
	}

	// Wait for exactly semSize requests to be in-flight inside the handler.
	for range semSize {
		select {
		case <-reached:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for requests to reach server")
		}
	}

	assert.EqualValues(t, semSize, currentInFlight.Load())

	close(release)
	wg.Wait()
}

// TestFetchCredentialsForRole_ContextCancelledWaitingForSemaphore verifies
// that a cancelled context while waiting for a semaphore slot is returned
// immediately as an error.
func TestFetchCredentialsForRole_ContextCancelledWaitingForSemaphore(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	fullSem := make(chan struct{}, 5)
	for range 5 {
		fullSem <- struct{}{}
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	s3Config := &cacheconfig.CacheS3Config{
		AccessKey:          "test-access-key",
		SecretKey:          "test-secret-key",
		AuthenticationType: "access-key",
		BucketName:         bucketName,
		BucketLocation:     "us-east-1",
	}
	client, err := newS3Client(s3Config, withSTSEndpoint("http://127.0.0.1:0/sts"), withAssumeRoleSem(fullSem))
	require.NoError(t, err)

	_, err = client.FetchCredentialsForRole(ctx, "arn:aws:iam::123456789012:role/TestRole", bucketName, objectName, true, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled waiting for AssumeRole semaphore")
}

// TestFetchCredentialsForRole_Metrics verifies that a successful call updates
// the in-flight gauge and records an observation in both duration histograms.
func TestFetchCredentialsForRole_Metrics(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	origInFlight := assumeRoleInFlight
	origWait := assumeRoleWaitDuration
	origCall := assumeRoleCallDuration
	testInFlight := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_in_flight", Help: "test"})
	testWait := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_wait", Help: "test"})
	testCall := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_call", Help: "test"})
	assumeRoleInFlight = testInFlight
	assumeRoleWaitDuration = testWait
	assumeRoleCallDuration = testCall
	t.Cleanup(func() {
		assumeRoleInFlight = origInFlight
		assumeRoleWaitDuration = origWait
		assumeRoleCallDuration = origCall
	})

	mockServer := httptest.NewServer(newMockSTSHandler(false, 3600, ""))
	defer mockServer.Close()

	s3Config := &cacheconfig.CacheS3Config{
		AccessKey:          "test-access-key",
		SecretKey:          "test-secret-key",
		AuthenticationType: "access-key",
		BucketName:         bucketName,
	}
	client, err := newS3Client(s3Config, withSTSEndpoint(mockServer.URL+"/sts"))
	require.NoError(t, err)

	_, err = client.FetchCredentialsForRole(t.Context(), "arn:aws:iam::123456789012:role/TestRole", bucketName, objectName, true, 0)
	require.NoError(t, err)

	// In-flight gauge must return to 0 after the call completes.
	assert.EqualValues(t, 0, testutil.ToFloat64(testInFlight))
	// Both histograms must have recorded exactly one observation.
	assert.EqualValues(t, 1, histogramSampleCount(t, testWait))
	assert.EqualValues(t, 1, histogramSampleCount(t, testCall))
}

func TestDetectBucketLocation(t *testing.T) {
	tests := map[string]struct {
		locationConstraint string
		serverError        bool
		expectedLocation   string
	}{
		"returns region from custom endpoint": {
			locationConstraint: "us-west-2",
			expectedLocation:   "us-west-2",
		},
		"maps EU alias to eu-west-1": {
			locationConstraint: "EU",
			expectedLocation:   "eu-west-1",
		},
		"falls back to us-east-1 on server error": {
			serverError:      true,
			expectedLocation: fallbackBucketLocation,
		},
		"falls back to us-east-1 on empty location constraint": {
			locationConstraint: "",
			expectedLocation:   fallbackBucketLocation,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			// Provide static credentials so the SDK doesn't attempt IMDS lookups.
			t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")

			serverCalled := false
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serverCalled = true
				if tt.serverError {
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				// GetBucketLocation is a GET /<bucket>?location request.
				// Respond with the configured location constraint for any request.
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w,
					`<?xml version="1.0" encoding="UTF-8"?><LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">%s</LocationConstraint>`,
					tt.locationConstraint,
				)
			}))
			defer ts.Close()

			tsURL, err := url.Parse(ts.URL)
			require.NoError(t, err)

			s3Config := &cacheconfig.CacheS3Config{
				BucketName:    "test-bucket",
				ServerAddress: tsURL.Host,
				Insecure:      true,
			}

			location := detectBucketLocation(s3Config)
			assert.Equal(t, tt.expectedLocation, location)
			assert.True(t, serverCalled, "expected the mock server to be contacted")
		})
	}
}

// TestFetchCredentialsForRole_CacheHit verifies that a second call with the
// same (roleARN, bucketName, objectName, upload) tuple returns the cached
// credentials without issuing a new STS request.
func TestFetchCredentialsForRole_CacheHit(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <AccessKeyId>cached-key</AccessKeyId>
      <SecretAccessKey>cached-secret</SecretAccessKey>
      <SessionToken>cached-token</SessionToken>
      <Expiration>%s</Expiration>
    </Credentials>
  </AssumeRoleResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</AssumeRoleResponse>`, time.Now().Add(time.Hour).Format(time.RFC3339))
	}))
	defer server.Close()

	s3Config := &cacheconfig.CacheS3Config{
		AccessKey:          "test-access-key",
		SecretKey:          "test-secret-key",
		AuthenticationType: "access-key",
		BucketName:         bucketName,
		BucketLocation:     "us-east-1",
	}
	roleARN := "arn:aws:iam::123456789012:role/CacheTestRole"

	client, err := newS3Client(s3Config, withSTSEndpoint(server.URL))
	require.NoError(t, err)

	// First call: hits STS.
	creds1, err := client.FetchCredentialsForRole(t.Context(), roleARN, bucketName, objectName, false, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 1, callCount.Load(), "first call should reach STS")

	// Second call with the same key: must return the cached creds, not call STS again.
	creds2, err := client.FetchCredentialsForRole(t.Context(), roleARN, bucketName, objectName, false, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 1, callCount.Load(), "second call must be served from cache")
	assert.Equal(t, creds1, creds2)

	// A call with a different key (upload=true) must reach STS.
	_, err = client.FetchCredentialsForRole(t.Context(), roleARN, bucketName, objectName, true, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 2, callCount.Load(), "different key must reach STS")
}

// TestFetchCredentialsForRole_CacheExpiry verifies that a cached credential
// that does not have enough remaining validity is not reused.
func TestFetchCredentialsForRole_CacheExpiry(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	// Pre-populate the cache with credentials that expire in 30 seconds —
	// less than the 1-minute minimum validity floor.
	credKey := assumeRoleCacheKey("arn:aws:iam::123456789012:role/ExpiryRole", bucketName, objectName, false)
	assumeRoleCredCache.Add(credKey, cachedCredential{
		creds:     map[string]string{"AWS_ACCESS_KEY_ID": "stale-key"},
		expiresAt: time.Now().Add(30 * time.Second),
	})

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <AccessKeyId>fresh-key</AccessKeyId>
      <SecretAccessKey>fresh-secret</SecretAccessKey>
      <SessionToken>fresh-token</SessionToken>
      <Expiration>%s</Expiration>
    </Credentials>
  </AssumeRoleResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</AssumeRoleResponse>`, time.Now().Add(time.Hour).Format(time.RFC3339))
	}))
	defer server.Close()

	s3Config := &cacheconfig.CacheS3Config{
		AccessKey:          "test-access-key",
		SecretKey:          "test-secret-key",
		AuthenticationType: "access-key",
		BucketName:         bucketName,
		BucketLocation:     "us-east-1",
	}
	client, err := newS3Client(s3Config, withSTSEndpoint(server.URL))
	require.NoError(t, err)

	creds, err := client.FetchCredentialsForRole(t.Context(), "arn:aws:iam::123456789012:role/ExpiryRole", bucketName, objectName, false, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 1, callCount.Load(), "expired cache entry must not be reused")
	assert.Equal(t, "fresh-key", creds["AWS_ACCESS_KEY_ID"])
}

// TestFetchCredentialsForRole_NoErrorCaching verifies that a failed AssumeRole
// call does not populate the cache.
func TestFetchCredentialsForRole_NoErrorCaching(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	// Use an unreachable STS endpoint to force an error.
	s3Config := &cacheconfig.CacheS3Config{
		AccessKey:          "test-access-key",
		SecretKey:          "test-secret-key",
		AuthenticationType: "access-key",
		BucketName:         bucketName,
		BucketLocation:     "us-east-1",
	}
	roleARN := "arn:aws:iam::123456789012:role/ErrorRole"
	client, err := newS3Client(s3Config, withSTSEndpoint("http://127.0.0.1:0"))
	require.NoError(t, err)

	_, err = client.FetchCredentialsForRole(t.Context(), roleARN, bucketName, objectName, false, 0)
	require.Error(t, err)

	// The cache must not contain an entry for the failed key.
	credKey := assumeRoleCacheKey(roleARN, bucketName, objectName, false)
	_, cached := assumeRoleCredCache.Get(credKey)
	assert.False(t, cached, "failed AssumeRole call must not be cached")
}

// TestFlushCredentialCache verifies that FlushCredentialCache removes all
// entries regardless of their validity.
func TestFlushCredentialCache(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	assumeRoleCredCache.Add("key-a", cachedCredential{
		creds:     map[string]string{"k": "v"},
		expiresAt: time.Now().Add(time.Hour),
	})
	assumeRoleCredCache.Add("key-b", cachedCredential{
		creds:     map[string]string{"k": "v"},
		expiresAt: time.Now().Add(time.Hour),
	})
	require.Equal(t, 2, assumeRoleCredCache.Len())

	FlushCredentialCache()

	assert.Equal(t, 0, assumeRoleCredCache.Len())
}

// TestFetchCredentialsForRole_CacheMetrics verifies that cache hits and misses
// are counted correctly.
func TestFetchCredentialsForRole_CacheMetrics(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	origHits := assumeRoleCredCacheHits
	origMisses := assumeRoleCredCacheMisses
	testHits := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_cache_hits", Help: "test"})
	testMisses := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_cache_misses", Help: "test"})
	assumeRoleCredCacheHits = testHits
	assumeRoleCredCacheMisses = testMisses
	t.Cleanup(func() {
		assumeRoleCredCacheHits = origHits
		assumeRoleCredCacheMisses = origMisses
	})

	server := httptest.NewServer(newMockSTSHandler(false, 3600, ""))
	defer server.Close()

	s3Config := &cacheconfig.CacheS3Config{
		AccessKey:          "test-access-key",
		SecretKey:          "test-secret-key",
		AuthenticationType: "access-key",
		BucketName:         bucketName,
		BucketLocation:     "us-east-1",
	}
	roleARN := "arn:aws:iam::123456789012:role/TestRole"
	client, err := newS3Client(s3Config, withSTSEndpoint(server.URL+"/sts"))
	require.NoError(t, err)

	// First call: cache miss.
	_, err = client.FetchCredentialsForRole(t.Context(), roleARN, bucketName, objectName, true, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 0, testutil.ToFloat64(testHits))
	assert.EqualValues(t, 1, testutil.ToFloat64(testMisses))

	// Second call with the same key: cache hit.
	_, err = client.FetchCredentialsForRole(t.Context(), roleARN, bucketName, objectName, true, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 1, testutil.ToFloat64(testHits))
	assert.EqualValues(t, 1, testutil.ToFloat64(testMisses))
}

// TestFetchCredentialsForRole_CacheDisabled verifies that setting
// DisableAssumeRoleCredentialsCaching causes every call to reach STS.
func TestFetchCredentialsForRole_CacheDisabled(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <AccessKeyId>key</AccessKeyId>
      <SecretAccessKey>secret</SecretAccessKey>
      <SessionToken>token</SessionToken>
      <Expiration>%s</Expiration>
    </Credentials>
  </AssumeRoleResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</AssumeRoleResponse>`, time.Now().Add(time.Hour).Format(time.RFC3339))
	}))
	defer server.Close()

	s3Config := &cacheconfig.CacheS3Config{
		AccessKey:                           "test-access-key",
		SecretKey:                           "test-secret-key",
		AuthenticationType:                  "access-key",
		BucketName:                          bucketName,
		BucketLocation:                      "us-east-1",
		DisableAssumeRoleCredentialsCaching: true,
	}
	roleARN := "arn:aws:iam::123456789012:role/TestRole"
	client, err := newS3Client(s3Config, withSTSEndpoint(server.URL))
	require.NoError(t, err)

	for range 3 {
		_, err = client.FetchCredentialsForRole(t.Context(), roleARN, bucketName, objectName, true, 0)
		require.NoError(t, err)
	}

	assert.EqualValues(t, 3, callCount.Load(), "every call must reach STS when caching is disabled")
	_, cached := assumeRoleCredCache.Get(assumeRoleCacheKey(roleARN, bucketName, objectName, true))
	assert.False(t, cached, "disabled cache must not be populated")
}

func TestFetchCredentialsForRole_FailureMetric(t *testing.T) {
	FlushCredentialCache()
	t.Cleanup(FlushCredentialCache)

	origFailures := assumeRoleFailures
	t.Cleanup(func() { assumeRoleFailures = origFailures })

	t.Run("STS error increments counter", func(t *testing.T) {
		testFailures := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_failures_sts", Help: "test"})
		assumeRoleFailures = testFailures

		s3Config := &cacheconfig.CacheS3Config{
			AccessKey:          "test-access-key",
			SecretKey:          "test-secret-key",
			AuthenticationType: "access-key",
			BucketName:         bucketName,
			BucketLocation:     "us-east-1",
		}
		client, err := newS3Client(s3Config, withSTSEndpoint("http://127.0.0.1:0"))
		require.NoError(t, err)

		_, err = client.FetchCredentialsForRole(t.Context(), "arn:aws:iam::123456789012:role/TestRole", bucketName, objectName, true, 0)
		require.Error(t, err)
		assert.EqualValues(t, 1, testutil.ToFloat64(testFailures))
	})

	t.Run("nil credentials increments counter", func(t *testing.T) {
		testFailures := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_failures_nil", Help: "test"})
		assumeRoleFailures = testFailures

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <AssumedRoleUser>
      <AssumedRoleId>AROATEST123:TestSession</AssumedRoleId>
      <Arn>arn:aws:sts::123456789012:assumed-role/TestRole/TestSession</Arn>
    </AssumedRoleUser>
  </AssumeRoleResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</AssumeRoleResponse>`))
		}))
		defer server.Close()

		s3Config := &cacheconfig.CacheS3Config{
			AccessKey:          "test-access-key",
			SecretKey:          "test-secret-key",
			AuthenticationType: "access-key",
			BucketName:         bucketName,
			BucketLocation:     "us-east-1",
		}
		client, err := newS3Client(s3Config, withSTSEndpoint(server.URL+"/sts"))
		require.NoError(t, err)

		_, err = client.FetchCredentialsForRole(t.Context(), "arn:aws:iam::123456789012:role/TestRole", bucketName, objectName, true, 0)
		require.Error(t, err)
		assert.EqualValues(t, 1, testutil.ToFloat64(testFailures))
	})
}
