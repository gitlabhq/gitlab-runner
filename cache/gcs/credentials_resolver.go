package gcs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"cloud.google.com/go/compute/metadata"
	credentialsapiv1 "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

//go:generate mockery --name=credentialsResolver --inpackage
type credentialsResolver interface {
	Credentials() *common.CacheGCSCredentials
	Resolve() error
	SignBytesFunc() func([]byte) ([]byte, error)
}

//go:generate mockery --name=IamCredentialsClient --inpackage
type IamCredentialsClient interface {
	SignBlob(
		context.Context,
		*credentialspb.SignBlobRequest,
		...gax.CallOption,
	) (*credentialspb.SignBlobResponse, error)
}

//go:generate mockery --name=MetadataClient --inpackage
type MetadataClient interface {
	Email(serviceAccount string) (string, error)
}

const TypeServiceAccount = "service_account"

type credentialsFile struct {
	Type        string `json:"type"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
}

type defaultCredentialsResolver struct {
	config            *common.CacheGCSConfig
	credentials       *common.CacheGCSCredentials
	metadataClient    MetadataClient
	credentialsClient IamCredentialsClient
}

func (cr *defaultCredentialsResolver) Credentials() *common.CacheGCSCredentials {
	return cr.credentials
}

func (cr *defaultCredentialsResolver) Resolve() error {
	if cr.config.CredentialsFile != "" {
		return cr.readCredentialsFromFile()
	}
	if cr.config.AccessID == "" && cr.config.PrivateKey == "" {
		return cr.readAccessIDFromMetadataServer()
	}

	return cr.readCredentialsFromConfig()
}

func (cr *defaultCredentialsResolver) SignBytesFunc() func([]byte) ([]byte, error) {
	return func(payload []byte) ([]byte, error) {
		ctx := context.Background()
		req := &credentialspb.SignBlobRequest{
			Name:    cr.credentials.AccessID,
			Payload: payload,
		}

		client, err := cr.iamCredentialsClient(ctx)
		if err != nil {
			return nil, err
		}

		res, err := client.SignBlob(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("signing blob: %w", err)
		}

		return res.SignedBlob, nil
	}
}

func (cr *defaultCredentialsResolver) readCredentialsFromFile() error {
	data, err := os.ReadFile(cr.config.CredentialsFile)
	if err != nil {
		return fmt.Errorf("error while reading credentials file: %w", err)
	}

	var credentialsFileContent credentialsFile
	err = json.Unmarshal(data, &credentialsFileContent)
	if err != nil {
		return fmt.Errorf("error while parsing credentials file: %w", err)
	}

	if credentialsFileContent.Type != TypeServiceAccount {
		return fmt.Errorf("unsupported credentials file type: %s", credentialsFileContent.Type)
	}

	logrus.Debugln("Credentials loaded from file. Skipping direct settings from Runner configuration file")

	cr.credentials.AccessID = credentialsFileContent.ClientEmail
	cr.credentials.PrivateKey = credentialsFileContent.PrivateKey

	return nil
}

func (cr *defaultCredentialsResolver) readCredentialsFromConfig() error {
	if cr.config.AccessID == "" || cr.config.PrivateKey == "" {
		return fmt.Errorf("GCS config present, but credentials are not configured")
	}

	cr.credentials.AccessID = cr.config.AccessID
	cr.credentials.PrivateKey = cr.config.PrivateKey

	return nil
}

func (cr *defaultCredentialsResolver) readAccessIDFromMetadataServer() error {
	email, err := cr.metadataClient.Email("")
	if err != nil {
		return fmt.Errorf("getting email from metadata server: %w", err)
	}
	cr.credentials.AccessID = email
	return nil
}

func (cr *defaultCredentialsResolver) iamCredentialsClient(ctx context.Context) (IamCredentialsClient, error) {
	if cr.credentialsClient == nil {
		var err error
		cr.credentialsClient, err = credentialsapiv1.NewIamCredentialsClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("creating iam credentials client: %w", err)
		}
	}

	return cr.credentialsClient, nil
}

func newDefaultCredentialsResolver(config *common.CacheGCSConfig) (*defaultCredentialsResolver, error) {
	if config == nil {
		return nil, fmt.Errorf("config can't be nil")
	}

	credentials := &defaultCredentialsResolver{
		config:         config,
		credentials:    &common.CacheGCSCredentials{},
		metadataClient: metadata.NewClient(nil),
	}

	return credentials, nil
}

var credentialsResolverInitializer = newDefaultCredentialsResolver
