package cacheconfig

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7/pkg/s3utils"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Type                   string `toml:"Type,omitempty" long:"type" env:"CACHE_TYPE" description:"Select caching method"`
	Path                   string `toml:"Path,omitempty" long:"path" env:"CACHE_PATH" description:"Name of the path to prepend to the cache URL"`
	Shared                 bool   `toml:"Shared,omitempty" long:"shared" env:"CACHE_SHARED" description:"Enable cache sharing between runners."`
	MaxUploadedArchiveSize int64  `toml:"MaxUploadedArchiveSize,omitempty" long:"max_uploaded_archive_size" env:"CACHE_MAXIMUM_UPLOADED_ARCHIVE_SIZE" description:"Limit the size of the cache archive being uploaded to cloud storage, in bytes."`

	S3    *CacheS3Config    `toml:"s3,omitempty" json:"s3,omitempty" namespace:"s3"`
	GCS   *CacheGCSConfig   `toml:"gcs,omitempty" json:"gcs,omitempty" namespace:"gcs"`
	Azure *CacheAzureConfig `toml:"azure,omitempty" json:"azure,omitempty" namespace:"azure"`
}

func (c *Config) GetPath() string {
	return c.Path
}

func (c *Config) GetShared() bool {
	return c.Shared
}

type CacheS3Config struct {
	ServerAddress             string     `toml:"ServerAddress,omitempty" long:"server-address" env:"CACHE_S3_SERVER_ADDRESS" description:"A host:port to the used S3-compatible server"`
	AccessKey                 string     `toml:"AccessKey,omitempty" long:"access-key" env:"CACHE_S3_ACCESS_KEY" description:"S3 Access Key"`
	SecretKey                 string     `toml:"SecretKey,omitempty" long:"secret-key" env:"CACHE_S3_SECRET_KEY" description:"S3 Secret Key"`
	SessionToken              string     `toml:"SessionToken,omitempty" long:"session-token" env:"CACHE_S3_SESSION_TOKEN" description:"S3 Session Token"`
	BucketName                string     `toml:"BucketName,omitempty" long:"bucket-name" env:"CACHE_S3_BUCKET_NAME" description:"Name of the bucket where cache will be stored"`
	BucketLocation            string     `toml:"BucketLocation,omitempty" long:"bucket-location" env:"CACHE_S3_BUCKET_LOCATION" description:"Name of S3 region"`
	Insecure                  bool       `toml:"Insecure,omitempty" long:"insecure" env:"CACHE_S3_INSECURE" description:"Use insecure mode (without https)"`
	AuthenticationType        S3AuthType `toml:"AuthenticationType,omitempty" long:"authentication_type" env:"CACHE_S3_AUTHENTICATION_TYPE" description:"IAM or credentials"`
	ServerSideEncryption      string     `toml:"ServerSideEncryption,omitempty" long:"server-side-encryption" env:"CACHE_S3_SERVER_SIDE_ENCRYPTION" description:"Server side encryption type (S3, or KMS)"`
	ServerSideEncryptionKeyID string     `toml:"ServerSideEncryptionKeyID,omitempty" long:"server-side-encryption-key-id" env:"CACHE_S3_SERVER_SIDE_ENCRYPTION_KEY_ID" description:"Server side encryption key ID (alias or Key ID or Key ARN)"`
	DualStack                 *bool      `toml:"DualStack,omitempty" long:"dual-stack" env:"CACHE_S3_DUAL_STACK" description:"Enable dual-stack (IPv4 and IPv6) endpoints (default: true)" jsonschema:"oneof_type=boolean;null"`
	PathStyle                 *bool      `toml:"PathStyle,omitempty" long:"path-style" env:"CACHE_S3_PATH_STYLE" description:"Use path style access (default: false)" jsonschema:"oneof_type=boolean;null"`
	Accelerate                bool       `toml:"Accelerate,omitempty" long:"accelerate" env:"CACHE_S3_ACCELERATE" description:"Enable S3 Transfer Acceleration"`
	RoleARN                   string     `toml:"RoleARN,omitempty" long:"role-arn" env:"CACHE_S3_ROLE_ARN" description:"Role ARN for transferring cache to S3"`
	UploadRoleARN             string     `toml:"UploadRoleARN,omitempty" long:"upload-role-arn" env:"CACHE_S3_UPLOAD_ROLE_ARN" description:"Role ARN for uploading cache to S3"`
}

type S3AuthType string

const (
	S3AuthTypeAccessKey S3AuthType = "access-key"
	S3AuthTypeIAM       S3AuthType = "iam"
)

type S3EncryptionType string

const (
	S3EncryptionTypeNone    S3EncryptionType = ""
	S3EncryptionTypeAes256  S3EncryptionType = "S3"
	S3EncryptionTypeKms     S3EncryptionType = "KMS"
	S3EncryptionTypeDsseKms S3EncryptionType = "DSSE-KMS"
)

func (c *CacheS3Config) AuthType() S3AuthType {
	authType := S3AuthType(strings.ToLower(string(c.AuthenticationType)))

	switch authType {
	case S3AuthTypeAccessKey, S3AuthTypeIAM:
		return authType
	}

	if authType != "" {
		return ""
	}

	if c.ServerAddress == "" || c.AccessKey == "" || c.SecretKey == "" {
		return S3AuthTypeIAM
	}

	return S3AuthTypeAccessKey
}

func (c *CacheS3Config) EncryptionType() S3EncryptionType {
	encryptionType := S3EncryptionType(strings.ToUpper(c.ServerSideEncryption))

	switch encryptionType {
	case "":
		return S3EncryptionTypeNone
	case "S3", "AES256":
		return S3EncryptionTypeAes256
	case "KMS", "AWS:KMS":
		return S3EncryptionTypeKms
	case "DSSE-KMS", "AWS:KMS:DSSE":
		return S3EncryptionTypeDsseKms
	}

	logrus.Warnf("unknown ServerSideEncryption value: %s", encryptionType)
	return S3EncryptionTypeNone
}

func (c *CacheS3Config) GetEndpoint() string {
	if c.ServerAddress == "" {
		return ""
	}

	scheme := "https"
	if c.Insecure {
		scheme = "http"
	}

	host, port, err := net.SplitHostPort(c.ServerAddress)
	if err != nil {
		// If SplitHostPort fails, it means there's no port specified
		// so we can use the ServerAddress as-is.
		return fmt.Sprintf("%s://%s", scheme, c.ServerAddress)
	}

	// Omit canonical ports
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		return fmt.Sprintf("%s://%s", scheme, host)
	}

	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

func (c *CacheS3Config) GetEndpointURL() *url.URL {
	endpoint := c.GetEndpoint()
	if endpoint == "" {
		return nil
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		logrus.Errorf("error parsing endpoint URL: %v", err)
		return nil
	}

	return u
}

// PathStyleEnabled() will return true if the endpoint needs to use
// the legacy, path-style access to S3. If the value is not specified,
// it will auto-detect and return false if the server address appears
// to be for AWS or Google. Otherwise, PathStyleEnabled() will return false.
func (c *CacheS3Config) PathStyleEnabled() bool {
	// Preserve the previous behavior of auto-detection by default
	if c.PathStyle == nil {
		u := c.GetEndpointURL()
		if u == nil {
			return false
		}

		return !s3utils.IsVirtualHostSupported(*u, c.BucketName)
	}

	return *c.PathStyle
}

func (c *CacheS3Config) DualStackEnabled() bool {
	if c.DualStack == nil {
		return true
	}
	return *c.DualStack
}

type CacheGCSCredentials struct {
	AccessID   string `toml:"AccessID,omitempty" long:"access-id" env:"CACHE_GCS_ACCESS_ID" description:"ID of GCP Service Account used to access the storage"`
	PrivateKey string `toml:"PrivateKey,omitempty" long:"private-key" env:"CACHE_GCS_PRIVATE_KEY" description:"Private key used to sign GCS requests"`
}

type CacheGCSConfig struct {
	CacheGCSCredentials
	CredentialsFile string `toml:"CredentialsFile,omitempty" long:"credentials-file" env:"GOOGLE_APPLICATION_CREDENTIALS" description:"File with GCP credentials, containing AccessID and PrivateKey"`
	BucketName      string `toml:"BucketName,omitempty" long:"bucket-name" env:"CACHE_GCS_BUCKET_NAME" description:"Name of the bucket where cache will be stored"`
	UniverseDomain  string `toml:"UniverseDomain,omitempty" long:"universe-domain" env:"CACHE_GCS_UNIVERSE_DOMAIN" description:"Universe Domain for GCS requests (e.g., googleapis.com for public cloud, or a custom universe domain)"`
}

type CacheAzureCredentials struct {
	AccountName string `toml:"AccountName,omitempty" long:"account-name" env:"CACHE_AZURE_ACCOUNT_NAME" description:"Account name for Azure Blob Storage"`
	AccountKey  string `toml:"AccountKey,omitempty" long:"account-key" env:"CACHE_AZURE_ACCOUNT_KEY" description:"Access key for Azure Blob Storage"`
}

type CacheAzureConfig struct {
	CacheAzureCredentials
	ContainerName string `toml:"ContainerName,omitempty" long:"container-name" env:"CACHE_AZURE_CONTAINER_NAME" description:"Name of the Azure container where cache will be stored"`
	StorageDomain string `toml:"StorageDomain,omitempty" long:"storage-domain" env:"CACHE_AZURE_STORAGE_DOMAIN" description:"Domain name of the Azure storage (e.g. blob.core.windows.net)"`
}
