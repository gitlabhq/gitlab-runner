package gcs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type credentialsResolver interface {
	Credentials() *common.CacheGCSCredentials
	Resolve() error
}

const TypeServiceAccount = "service_account"

type credentialsFile struct {
	Type        string `json:"type"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
}

type defaultCredentialsResolver struct {
	config      *common.CacheGCSConfig
	credentials *common.CacheGCSCredentials
}

func (cr *defaultCredentialsResolver) Credentials() *common.CacheGCSCredentials {
	return cr.credentials
}

func (cr *defaultCredentialsResolver) Resolve() error {
	if cr.config.CredentialsFile != "" {
		return cr.readCredentialsFromFile()
	}

	return cr.readCredentialsFromConfig()
}

func (cr *defaultCredentialsResolver) readCredentialsFromFile() error {
	data, err := ioutil.ReadFile(cr.config.CredentialsFile)
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

func newDefaultCredentialsResolver(config *common.CacheGCSConfig) (*defaultCredentialsResolver, error) {
	if config == nil {
		return nil, fmt.Errorf("config can't be nil")
	}

	credentials := &defaultCredentialsResolver{
		config:      config,
		credentials: &common.CacheGCSCredentials{},
	}

	return credentials, nil
}

var credentialsResolverInitializer = newDefaultCredentialsResolver
