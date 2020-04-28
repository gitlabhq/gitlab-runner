package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/homedir"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	// DefaultDockerRegistry is the name of the index
	DefaultDockerRegistry            = "docker.io"
	authConfigSourceNameUserVariable = "$DOCKER_AUTH_CONFIG"
	authConfigSourceNameJobPayload   = "job payload (GitLab Registry)"
)

type authConfigResolver func() (string, map[string]types.AuthConfig)

// GetAuthConfigForImage returns the auth configuration for a particular image.
// See GetAuthConfigs for source information.
func GetAuthConfigForImage(imageName, dockerAuthConfig, username string, credentials []common.Credentials) (string, *types.AuthConfig) {
	source, authConfigs := GetAuthConfigs(dockerAuthConfig, username, credentials)
	if authConfigs == nil {
		return source, nil
	}
	indexName, _ := SplitDockerImageName(imageName)
	return source, resolveDockerAuthConfig(indexName, authConfigs)
}

// GetAuthConfigs returns the authentication configuration for docker registries.
// Goes through several sources in this order:
// 1. DOCKER_AUTH_CONFIG
// 2. ~/.docker/config.json or .dockercfg
// 3. Build credentials
func GetAuthConfigs(dockerAuthConfig, username string, credentials []common.Credentials) (string, map[string]types.AuthConfig) {
	resolvers := []authConfigResolver{
		func() (string, map[string]types.AuthConfig) {
			return getUserAuthConfiguration(dockerAuthConfig)
		},
		func() (string, map[string]types.AuthConfig) {
			return getHomeDirAuthConfiguration(username)
		},
		func() (string, map[string]types.AuthConfig) {
			return getBuildAuthConfiguration(credentials)
		},
	}

	for _, resolver := range resolvers {
		source, authConfigs := resolver()

		if authConfigs != nil {
			return source, authConfigs
		}
	}
	return "", nil
}

func getUserAuthConfiguration(dockerAuthConfig string) (string, map[string]types.AuthConfig) {
	buf := bytes.NewBufferString(dockerAuthConfig)
	authConfigs, _ := readAuthConfigsFromReader(buf)

	if authConfigs == nil {
		return "", nil
	}

	return authConfigSourceNameUserVariable, authConfigs
}

func getBuildAuthConfiguration(credentials []common.Credentials) (string, map[string]types.AuthConfig) {
	authConfigs := make(map[string]types.AuthConfig)

	for _, credentials := range credentials {
		if credentials.Type != "registry" {
			continue
		}

		authConfigs[credentials.URL] = types.AuthConfig{
			Username:      credentials.Username,
			Password:      credentials.Password,
			ServerAddress: credentials.URL,
		}
	}

	return authConfigSourceNameJobPayload, authConfigs
}

func getHomeDirAuthConfiguration(username string) (string, map[string]types.AuthConfig) {
	sourceFile, authConfigs, _ := readDockerAuthConfigsFromHomeDir(username)

	if authConfigs == nil {
		return "", nil
	}
	return sourceFile, authConfigs
}

// EncodeAuthConfig constructs a token from an AuthConfig, suitable for
// authorizing against the Docker API with.
func EncodeAuthConfig(authConfig *types.AuthConfig) (string, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(authConfig); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

// SplitDockerImageName breaks a reposName into an index name and remote name
func SplitDockerImageName(reposName string) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	var indexName, remoteName string
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io'
		indexName = DefaultDockerRegistry
		remoteName = reposName
	} else {
		indexName = nameParts[0]
		remoteName = nameParts[1]
	}

	if indexName == "index."+DefaultDockerRegistry {
		indexName = DefaultDockerRegistry
	}
	return indexName, remoteName
}

var HomeDirectory = homedir.Get()
var errNoHomeDir = errors.New("no home directory found")

// readDockerAuthConfigsFromHomeDir reads known docker config from home
// directory. If no username is provided it will get the home directory for the
// current user.
func readDockerAuthConfigsFromHomeDir(userName string) (string, map[string]types.AuthConfig, error) {
	homeDir := HomeDirectory

	if userName != "" {
		u, err := user.Lookup(userName)
		if err != nil {
			return "", nil, err
		}
		homeDir = u.HomeDir
	}

	if homeDir == "" {
		return "", nil, errNoHomeDir
	}

	configFile := filepath.Join(homeDir, ".docker", "config.json")

	r, err := os.Open(configFile)
	if err != nil {
		configFile = filepath.Join(homeDir, ".dockercfg")
		r, err = os.Open(configFile)
		if err != nil && !os.IsNotExist(err) {
			return "", nil, err
		}
	}
	defer r.Close()

	if r == nil {
		return "", make(map[string]types.AuthConfig), nil
	}

	authConfigs, err := readAuthConfigsFromReader(r)

	return configFile, authConfigs, err
}

func readAuthConfigsFromReader(r io.Reader) (map[string]types.AuthConfig, error) {
	config := &configfile.ConfigFile{}

	if err := config.LoadFromReader(r); err != nil {
		return nil, err
	}

	auths := make(map[string]types.AuthConfig)
	addAll(auths, config.AuthConfigs)

	if config.CredentialsStore != "" {
		authsFromCredentialsStore, err := readAuthConfigsFromCredentialsStore(config)
		if err != nil {
			return nil, err
		}
		addAll(auths, authsFromCredentialsStore)
	}

	if config.CredentialHelpers != nil {
		authsFromCredentialsHelpers, err := readAuthConfigsFromCredentialsHelper(config)
		if err != nil {
			return nil, err
		}
		addAll(auths, authsFromCredentialsHelpers)
	}

	return auths, nil
}

func readAuthConfigsFromCredentialsStore(config *configfile.ConfigFile) (map[string]types.AuthConfig, error) {
	store := credentials.NewNativeStore(config, config.CredentialsStore)
	newAuths, err := store.GetAll()
	if err != nil {
		return nil, err
	}

	return newAuths, nil
}

func readAuthConfigsFromCredentialsHelper(config *configfile.ConfigFile) (map[string]types.AuthConfig, error) {
	helpersAuths := make(map[string]types.AuthConfig)

	for registry, helper := range config.CredentialHelpers {
		store := credentials.NewNativeStore(config, helper)

		newAuths, err := store.Get(registry)
		if err != nil {
			return nil, err
		}

		helpersAuths[registry] = newAuths
	}

	return helpersAuths, nil
}

func addAll(to, from map[string]types.AuthConfig) {
	for reg, ac := range from {
		to[reg] = ac
	}
}

// resolveDockerAuthConfig taken from: https://github.com/docker/docker/blob/master/registry/go
func resolveDockerAuthConfig(indexName string, configs map[string]types.AuthConfig) *types.AuthConfig {
	if configs == nil {
		return nil
	}

	convertToHostname := func(url string) string {
		stripped := url
		if strings.HasPrefix(url, "http://") {
			stripped = strings.Replace(url, "http://", "", 1)
		} else if strings.HasPrefix(url, "https://") {
			stripped = strings.Replace(url, "https://", "", 1)
		}

		nameParts := strings.SplitN(stripped, "/", 2)
		if nameParts[0] == "index."+DefaultDockerRegistry {
			return DefaultDockerRegistry
		}
		return nameParts[0]
	}

	// Maybe they have a legacy config file, we will iterate the keys converting
	// them to the new format and testing
	for registry, authConfig := range configs {
		if indexName == convertToHostname(registry) {
			return &authConfig
		}
	}

	// When all else fails, return an empty auth config
	return nil
}
