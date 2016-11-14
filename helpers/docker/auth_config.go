package docker_helpers

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"strings"

	"github.com/docker/docker/pkg/homedir"
	"github.com/fsouza/go-dockerclient"
)

// DefaultDockerRegistry is the name of the index
const DefaultDockerRegistry = "docker.io"

// SplitDockerImageName breaks a reposName into an index name and remote name
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

func readDockerAuthConfigsFromPath(directory string) (*docker.AuthConfigurations, error) {
	var r io.Reader
	var err error
	p := path.Join(directory, ".docker", "config.json")
	r, err = os.Open(p)
	if err != nil {
		p := path.Join(directory, ".dockercfg")
		r, err = os.Open(p)
		if err != nil {
			return nil, err
		}
	}
	return docker.NewAuthConfigurations(r)
}

func readDockerAuthConfigsFromString(authConfigs string) (*docker.AuthConfigurations, error) {
	return docker.NewAuthConfigurations(strings.NewReader(authConfigs))
}

// ResolveDockerAuthConfig taken from: https://github.com/docker/docker/blob/master/registry/auth.go
func resolveDockerAuthConfig(indexName string, configs *docker.AuthConfigurations) *docker.AuthConfiguration {
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
	for registry, authConfig := range configs.Configs {
		if indexName == convertToHostname(registry) {
			return &authConfig
		}
	}

	// When all else fails, return an empty auth config
	return nil
}

var ResolveHomeDir = func(userName string) (string, error) {
	homeDir := homedir.Get()
	if userName != "" {
		u, err := user.Lookup(userName)
		if err != nil {
			return "", err
		}
		homeDir = u.HomeDir
	}
	if homeDir == "" {
		return "", fmt.Errorf("Failed to get home directory")
	}

	return homeDir, nil
}

type AuthConfigResolver struct {
	authConfigs *docker.AuthConfigurations
}

func (r *AuthConfigResolver) ReadHomeDirectoryAuthConfig(userName string) error {
	path, err := ResolveHomeDir(userName)
	if (err != nil) {
		return err
	}

	authConfigs, err := readDockerAuthConfigsFromPath(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return err
	}

	r.appendAuthConfigs(authConfigs)

	return nil
}

func (r *AuthConfigResolver) ReadStringAuthConfig(authConfigString string) error {
	if authConfigString == "" {
		return nil
	}

	authConfigs, err := readDockerAuthConfigsFromString(authConfigString)
	if err != nil {
		return err
	}

	r.appendAuthConfigs(authConfigs)

	return nil
}

func (r *AuthConfigResolver) appendAuthConfigs(authConfigs *docker.AuthConfigurations) {
	for key, value := range authConfigs.Configs {
		r.authConfigs.Configs[key] = value
	}
}

func (r *AuthConfigResolver) ResolveAuthConfig(imageName string) (*docker.AuthConfiguration, string) {
	indexName, _ := splitDockerImageName(imageName)

	if len(r.authConfigs.Configs) < 1 {
		return &docker.AuthConfiguration{}, indexName
	}

	authConfig := resolveDockerAuthConfig(indexName, r.authConfigs)

	return authConfig, indexName
}

func NewAuthConfigResolver() *AuthConfigResolver {
	return &AuthConfigResolver{
		authConfigs: &docker.AuthConfigurations{
			Configs: make(map[string]docker.AuthConfiguration),
		},
	}
}
