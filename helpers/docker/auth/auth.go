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
	"github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/pkg/homedir"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	// DefaultDockerRegistry is the name of the index
	DefaultDockerRegistry            = "docker.io"
	authConfigSourceNameUserVariable = "$DOCKER_AUTH_CONFIG"
	authConfigSourceNameJobPayload   = "job payload (GitLab Registry)"
)

var (
	HomeDirectory    = homedir.Get()
	errNoHomeDir     = errors.New("no home directory found")
	errPathTraversal = errors.New("path traversal is not allowed")
)

// RegistryInfo represents the source and authentication for a given registry.
type RegistryInfo struct {
	Source     string
	AuthConfig types.AuthConfig
}

type authConfigResolver func() (string, map[string]types.AuthConfig, error)

// ResolveConfigForImage returns the auth configuration for a particular image.
// Returns nil on no config found.
// See ResolveConfigs for source information.
func ResolveConfigForImage(
	imageName, dockerAuthConfig, username string,
	credentials []common.Credentials,
) (*RegistryInfo, error) {
	authConfigs, err := ResolveConfigs(dockerAuthConfig, username, credentials)
	if len(authConfigs) == 0 || err != nil {
		return nil, err
	}

	indexName, _ := splitDockerImageName(imageName)
	info, ok := authConfigs[indexName]
	if !ok {
		return nil, nil
	}

	return &info, nil
}

// ResolveConfigs returns the authentication configuration for docker registries.
// Goes through several sources in this order:
// 1. DOCKER_AUTH_CONFIG
// 2. ~/.docker/config.json or .dockercfg
// 3. Build credentials
// Returns a map of registry hostname to RegistryInfo
func ResolveConfigs(
	dockerAuthConfig, username string,
	credentials []common.Credentials,
) (map[string]RegistryInfo, error) {
	resolvers := []authConfigResolver{
		func() (string, map[string]types.AuthConfig, error) {
			return getUserConfiguration(dockerAuthConfig)
		},
		func() (string, map[string]types.AuthConfig, error) {
			return getHomeDirConfiguration(username)
		},
		func() (string, map[string]types.AuthConfig, error) {
			return getBuildConfiguration(credentials)
		},
	}
	res := make(map[string]RegistryInfo)

	for _, r := range resolvers {
		source, configs, err := r()
		if errors.Is(err, errPathTraversal) {
			return nil, err
		}

		for registry, conf := range configs {
			registryHostname := convertToHostname(registry)
			if _, ok := res[registryHostname]; !ok {
				res[registryHostname] = RegistryInfo{
					Source:     source,
					AuthConfig: conf,
				}
			}
		}
	}

	return res, nil
}

func getUserConfiguration(dockerAuthConfig string) (string, map[string]types.AuthConfig, error) {
	authConfigs, err := readConfigsFromReader(bytes.NewBufferString(dockerAuthConfig))
	if errors.Is(err, errPathTraversal) {
		return "", nil, err
	}
	if authConfigs == nil {
		return "", nil, nil
	}

	return authConfigSourceNameUserVariable, authConfigs, nil
}

func getHomeDirConfiguration(username string) (string, map[string]types.AuthConfig, error) {
	sourceFile, authConfigs, err := readDockerConfigsFromHomeDir(username)
	if errors.Is(err, errPathTraversal) {
		return "", nil, err
	}
	if authConfigs == nil {
		return "", nil, nil
	}

	return sourceFile, authConfigs, nil
}

// EncodeConfig constructs a token from an AuthConfig, suitable for
// authorizing against the Docker API with.
func EncodeConfig(authConfig *types.AuthConfig) (string, error) {
	if authConfig == nil {
		return "", nil
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(authConfig); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

func getBuildConfiguration(credentials []common.Credentials) (string, map[string]types.AuthConfig, error) {
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

	return authConfigSourceNameJobPayload, authConfigs, nil
}

// splitDockerImageName breaks a reposName into an index name and remote name
func splitDockerImageName(reposName string) (string, string) {
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

// readDockerConfigsFromHomeDir reads known docker config from home
// directory. If no username is provided it will get the home directory for the
// current user.
func readDockerConfigsFromHomeDir(userName string) (string, map[string]types.AuthConfig, error) {
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

	authConfigs, err := readConfigsFromReader(r)

	return configFile, authConfigs, err
}

func readConfigsFromReader(r io.Reader) (map[string]types.AuthConfig, error) {
	config := &configfile.ConfigFile{}

	if err := config.LoadFromReader(r); err != nil {
		if errors.Is(err, io.EOF) {
			err = nil
		}
		return nil, err
	}

	auths := make(map[string]types.AuthConfig)
	addAll(auths, config.AuthConfigs)

	if config.CredentialsStore != "" {
		authsFromCredentialsStore, err := readConfigsFromCredentialsStore(config)
		if err != nil {
			return nil, err
		}
		addAll(auths, authsFromCredentialsStore)
	}

	if config.CredentialHelpers != nil {
		authsFromCredentialsHelpers, err := readConfigsFromCredentialsHelper(config)
		if err != nil {
			return nil, err
		}
		addAll(auths, authsFromCredentialsHelpers)
	}

	return auths, nil
}

func readConfigsFromCredentialsStore(config *configfile.ConfigFile) (map[string]types.AuthConfig, error) {
	if config.CredentialsStore != filepath.Base(config.CredentialsStore) {
		// Fail processing if credential store attempting path traversal are detected
		return nil, errPathTraversal
	}

	store := credentials.NewNativeStore(config, config.CredentialsStore)
	newAuths, err := store.GetAll()
	if err != nil {
		return nil, err
	}

	return newAuths, nil
}

func readConfigsFromCredentialsHelper(config *configfile.ConfigFile) (map[string]types.AuthConfig, error) {
	helpersAuths := make(map[string]types.AuthConfig)

	for registry, helper := range config.CredentialHelpers {
		if helper != filepath.Base(helper) {
			// Fail processing if credential helpers attempting path traversal are detected
			return nil, errPathTraversal
		}

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

func convertToHostname(url string) string {
	url = strings.ToLower(url)
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")

	nameParts := strings.SplitN(url, "/", 2)
	url = nameParts[0]

	if url == "index."+DefaultDockerRegistry {
		return DefaultDockerRegistry
	}

	return url
}
