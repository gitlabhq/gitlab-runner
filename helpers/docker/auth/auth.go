package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"slices"
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
	errNoHomeDir     = errors.New("no home directory found")
	errPathTraversal = errors.New("path traversal is not allowed")
)

// RegistryInfo represents the source, normalized registry path and authentication for a registry.
type RegistryInfo struct {
	RegistryPath string
	Source       string
	AuthConfig   types.AuthConfig
}

// RegistryInfos is a list of RegistryInfo, with a stable order
type RegistryInfos []RegistryInfo

// Get returns a RegistryInfo, matching the registry path.
func (r RegistryInfos) Get(path string) (RegistryInfo, bool) {
	for _, i := range r {
		if i.RegistryPath == path {
			return i, true
		}
	}
	return RegistryInfo{}, false
}

// Add adds a RegistryInfo to the list of known registries. If a RegistryInfo for the same registry path exists already,
// an error is returned and the RegistryInfo is not appended.
func (r *RegistryInfos) Add(newInfo RegistryInfo) error {
	for _, existingInfo := range *r {
		if existingInfo.RegistryPath == newInfo.RegistryPath {
			return fmt.Errorf("credentials for %q already set from %q, ignoring credentials from %q", existingInfo.RegistryPath, existingInfo.Source, newInfo.Source)
		}
	}
	*r = append(*r, newInfo)
	return nil
}

type DebugLogger interface {
	Debugln(args ...interface{})
}

// the parent directory of a path or ""
func parentPath(path string) string {
	index := strings.LastIndex(path, "/")
	if index == -1 {
		return ""
	}
	return path[:index]
}

// Resolver provides mechanisms to get all known registry's and their auth, and specific ones for specific images.
type Resolver struct {
	HomeDirGetter func() string
}

func NewResolver() Resolver {
	return Resolver{
		HomeDirGetter: homedir.Get,
	}
}

// ConfigForImage returns the auth configuration for a particular image.
// It gets all configs via [AllConfigs] and returns the one with the longest match for imageName <-> RegistryInfo.RegistryPath
// It returns nil when no matching config can be found.
func (ar Resolver) ConfigForImage(
	imageName, dockerAuthConfig, username string,
	credentials []common.Credentials, logger DebugLogger,
) (*RegistryInfo, error) {
	authConfigs, err := ar.AllConfigs(dockerAuthConfig, username, credentials, logger)
	if len(authConfigs) == 0 || err != nil {
		return nil, err
	}

	path := normalizeImageRef(imageName)
	for p := path; p != ""; p = parentPath(p) {
		if info, ok := authConfigs.Get(p); ok {
			return &info, nil
		}
	}

	return nil, nil
}

// AllConfigs returns the authentication configuration for docker registries.
// Goes through several sources in this order:
// 1. DOCKER_AUTH_CONFIG
// 2. ~/.docker/config.json or .dockercfg
// 3. Build credentials
// Returns a list of RegistryInfos, in the order of discovery.
func (ar Resolver) AllConfigs(
	dockerAuthConfig, username string,
	credentials []common.Credentials, logger DebugLogger,
) (RegistryInfos, error) {
	resolvers := []func() (string, []types.AuthConfig, error){
		func() (string, []types.AuthConfig, error) {
			return getUserConfiguration(dockerAuthConfig)
		},
		func() (string, []types.AuthConfig, error) {
			return ar.getHomeDirConfiguration(username)
		},
		func() (string, []types.AuthConfig, error) {
			return getBuildConfiguration(credentials)
		},
	}
	res := RegistryInfos{}

	for _, r := range resolvers {
		source, configs, err := r()
		if errors.Is(err, errPathTraversal) {
			return nil, err
		}

		if len(configs) == 0 {
			continue
		}

		hostnames := []string{} // used only for logging

		for _, conf := range configs {
			registryPath := convertToRegistryPath(conf.ServerAddress)
			hostnames = append(hostnames, registryPath)

			newRegistryInfo := RegistryInfo{
				RegistryPath: registryPath,
				Source:       source,
				AuthConfig:   conf,
			}

			if err := res.Add(newRegistryInfo); err != nil {
				logger.Debugln(fmt.Sprintf("Not adding Docker credentials: %s", err.Error()))
			}
		}

		// Source can be blank if there is no home dir configuration
		if source != "" {
			logger.Debugln(fmt.Sprintf("Loaded Docker credentials, source = %q, hostnames = %v, error = %v", source, hostnames, err))
		}
	}

	return res, nil
}

func getUserConfiguration(dockerAuthConfig string) (string, []types.AuthConfig, error) {
	authConfigs, err := readConfigsFromReader(bytes.NewBufferString(dockerAuthConfig))
	if errors.Is(err, errPathTraversal) {
		return "", nil, err
	}
	if authConfigs == nil {
		return "", nil, nil
	}

	return authConfigSourceNameUserVariable, authConfigs, nil
}

func (ar Resolver) getHomeDirConfiguration(username string) (string, []types.AuthConfig, error) {
	sourceFile, authConfigs, err := ar.readDockerConfigsFromHomeDir(username)
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

func getBuildConfiguration(credentials []common.Credentials) (string, []types.AuthConfig, error) {
	authConfigs := make([]types.AuthConfig, 0, len(credentials))

	for _, credentials := range credentials {
		if credentials.Type != "registry" {
			continue
		}

		authConfigs = append(authConfigs, types.AuthConfig{
			Username:      credentials.Username,
			Password:      credentials.Password,
			ServerAddress: credentials.URL,
		})
	}

	return authConfigSourceNameJobPayload, authConfigs, nil
}

// normalizeImageRef takes a raw image reference and normalizes it:
//   - cuts off the tag
//   - normalizes docker.io image refs (nginx -> docker.io/nginx, index.docker.io/nginx -> docker.io/nginx)
//   - lower-cases the hostname
func normalizeImageRef(imageName string) string {
	// foo.bar.tld/blipo/blupp:latest -> [ foo.bar.tld/blipp/, blupp:latest ]
	dir, image := path.Split(imageName)

	// remove tag: blupp:latest -> blupp
	image, _, _ = strings.Cut(image, ":")

	// reconstruct again -> foo.bar.tld/blipo/blupp
	normalized := path.Join(dir, image)

	// foo.bar.tld/blipo/blupp -> [ foo.bar.tld, blipo/blupp ]
	nameParts := strings.SplitN(normalized, "/", 2)

	// is this an image from docker hub, like "nginx"?
	isDockerIO := len(nameParts) == 1 ||
		(!strings.Contains(nameParts[0], ".") &&
			!strings.Contains(nameParts[0], ":") &&
			!strings.EqualFold(nameParts[0], "localhost"))

	switch {
	case isDockerIO:
		// for docker.io images, explicitly prepend 'docker.io'
		normalized = path.Join(DefaultDockerRegistry, normalized)
	case strings.EqualFold(nameParts[0], "index."+DefaultDockerRegistry):
		// for 'index.docker.io' images, explicitly cut of the 'index.' part
		_, normalized, _ = strings.Cut(normalized, ".")
	}

	return pathWithLowerCaseHostname(normalized)
}

// readDockerConfigsFromHomeDir reads known docker config from home
// directory. If no username is provided it will get the home directory for the
// current user.
func (ar Resolver) readDockerConfigsFromHomeDir(userName string) (string, []types.AuthConfig, error) {
	homeDir := ar.HomeDirGetter()

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

	configFiles := []string{
		filepath.Join(homeDir, ".docker", "config.json"),
		filepath.Join(homeDir, ".dockercfg"),
	}

	var f *os.File
	var err error
	for _, fn := range configFiles {
		f, err = os.Open(fn)
		if err == nil {
			break
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return "", nil, err
	}
	if f == nil {
		return "", []types.AuthConfig{}, nil
	}
	defer f.Close()

	authConfigs, err := readConfigsFromReader(f)
	return f.Name(), authConfigs, err
}

func readConfigsFromReader(r io.Reader) ([]types.AuthConfig, error) {
	config := &configfile.ConfigFile{}
	if err := config.LoadFromReader(r); err != nil {
		return nil, err
	}
	if !config.ContainsAuth() {
		// we can bail out early when there is no auth configured at all
		return nil, nil
	}

	auths := config.GetAuthConfigs()

	if config.CredentialsStore != "" {
		authsFromCredentialsStore, err := readConfigsFromCredentialsStore(config)
		if err != nil {
			return nil, err
		}
		maps.Copy(auths, authsFromCredentialsStore)
	}

	if config.CredentialHelpers != nil {
		authsFromCredentialsHelpers, err := readConfigsFromCredentialsHelper(config)
		if err != nil {
			return nil, err
		}
		maps.Copy(auths, authsFromCredentialsHelpers)
	}

	return withStableOrder(auths), nil
}

// withStableOrder converts the map of AuthConfigs to a slice of AuthConfigs, ordered by the map's key.
// When parsing AuthConfigs from docker config files, the AuthConfig's ServerAddress is set to the same value as the
// map's key explicitly, so we can rely on that rather than the map's key.
func withStableOrder(acs map[string]types.AuthConfig) []types.AuthConfig {
	s := slices.Collect(maps.Keys(acs))
	slices.Sort(s)

	res := make([]types.AuthConfig, 0, len(s))
	for _, server := range s {
		res = append(res, acs[server])
	}

	return res
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

// convert hostname part to lower case.
// Since the hostname is case insensitive we convert it to lower case
// to allow matching with case sensitive comparison
func pathWithLowerCaseHostname(path string) string {
	nameParts := strings.SplitN(path, "/", 2)
	hostname := strings.ToLower(nameParts[0])
	if len(nameParts) == 1 {
		return hostname
	}

	return hostname + "/" + nameParts[1]
}

// Returns the normalized path for a docker registry reference for some credentials.
func convertToRegistryPath(imageRef string) string {
	protocol := regexp.MustCompile("(?i)^https?://")

	if protocol.MatchString(imageRef) {
		// old style with protocol and maybe suffix /v1/
		// just the use hostname
		path := protocol.ReplaceAllString(imageRef, "")

		nameParts := strings.SplitN(path, "/", 2)
		path = strings.ToLower(nameParts[0])

		if path == "index."+DefaultDockerRegistry {
			return DefaultDockerRegistry
		}

		return path
	}

	path := strings.TrimSuffix(imageRef, "/")

	tagIndex := strings.LastIndex(path, ":")
	pathIndex := strings.LastIndex(path, "/")
	// remove image tag from path
	if pathIndex != -1 && tagIndex > pathIndex {
		path = path[:strings.LastIndex(path, ":")]
	}

	return pathWithLowerCaseHostname(path)
}
