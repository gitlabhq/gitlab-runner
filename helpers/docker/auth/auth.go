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
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/docker/docker-credential-helpers/client"
	helpercredentials "github.com/docker/docker-credential-helpers/credentials"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/homedir"
)

const (
	// DefaultDockerRegistry is the name of the index
	DefaultDockerRegistry        = "docker.io"
	configSourceNameUserVariable = "$DOCKER_AUTH_CONFIG"
	configSourceNameJobPayload   = "job payload (GitLab Registry)"

	credHelperAllowlistDocsURL = "https://docs.gitlab.com/runner/configuration/advanced-configuration/#restrict-docker-credential-helpers"

	// credHelperPrefix is docker's naming convention for credential helper
	// binaries: a helper "foo" executes "docker-credential-foo".
	credHelperPrefix = "docker-credential-"

	// tokenUsername is the username docker credential helpers return when the
	// secret is an identity token rather than a password.
	tokenUsername = "<token>"
)

var (
	errNoHomeDir     = errors.New("no home directory found")
	errPathTraversal = errors.New("path traversal is not allowed")
)

// RegistryInfo represents the source, normalized registry path and authentication for a registry.
type RegistryInfo struct {
	Path       string
	Source     string
	AuthConfig types.AuthConfig
}

// RegistryInfos is a list of RegistryInfo, with a stable order
type RegistryInfos []RegistryInfo

// Get returns a RegistryInfo, matching the registry path.
func (ri RegistryInfos) Get(path string) (RegistryInfo, bool) {
	for _, i := range ri {
		if i.Path == path {
			return i, true
		}
	}
	return RegistryInfo{}, false
}

// Append adds a RegistryInfo to the list of known registries. If a RegistryInfo for the same registry path exists already,
// an error is returned and the RegistryInfo is not appended.
func (ri *RegistryInfos) Append(newInfo RegistryInfo) error {
	for _, existingInfo := range *ri {
		if existingInfo.Path == newInfo.Path {
			return fmt.Errorf("credentials for %q already set from %q, ignoring credentials from %q", existingInfo.Path, existingInfo.Source, newInfo.Source)
		}
	}
	*ri = append(*ri, newInfo)
	return nil
}

type Logger interface {
	Debugln(args ...any)
	Warningln(args ...any)
}

// the parent directory of a path or ""
func parentPath(path string) string {
	index := strings.LastIndex(path, "/")
	if index == -1 {
		return ""
	}
	return path[:index]
}

// homeDir wraps around docker's home dir getter while still allowing the implementation to be switched out
type homeDir func() string

func (hd homeDir) Get() string {
	if hd == nil {
		hd = homedir.New().Get
	}
	return hd()
}

// Resolver provides mechanisms to get all known registries and their auth, and specific ones for specific images.
type Resolver struct {
	homeDir homeDir

	// jobCredHelperAllowlist gates which docker credential helpers a
	// job-supplied DOCKER_AUTH_CONFIG can execute. See NewResolver.
	jobCredHelperAllowlist map[string][]string
}

// NewResolver returns a Resolver which executes a docker credential helper
// referenced by a job-supplied DOCKER_AUTH_CONFIG only when the allowlist
// lists the helper's name and one of that name's glob patterns matches the
// registry. An empty or nil allowlist runs no job-supplied helper. Helpers
// referenced by the runner host's own docker configuration files are
// operator-owned and never gated.
func NewResolver(jobCredHelperAllowlist map[string][]string) Resolver {
	return Resolver{jobCredHelperAllowlist: jobCredHelperAllowlist}
}

// allowedHelper is one parsed allowed_docker_credential_helpers entry.
type allowedHelper struct {
	key      string // the allowed_docker_credential_helpers key as the operator wrote it
	program  string // "docker-credential-<helper>" on PATH, or the operator-pinned absolute path
	patterns []string
}

// credHelperGate decides whether a credential helper referenced by a config
// source may run, which program backs it, and which registries a store may
// serve. Job-controlled sources use an allowlistGate; operator-owned sources
// (the runner host's own docker config) use ungated, which permits everything.
type credHelperGate interface {
	// permit reports whether helper may run for registry, logging any it skips.
	permit(helper, registry string, isStore bool) bool
	// program returns the executable backing helper.
	program(helper string) string
	// registryAllowed reports whether a permitted store may serve registry.
	registryAllowed(helper, registry string) bool
}

// ungated is an operator-owned source: every helper it references runs, backed
// by docker's PATH-resolved naming convention, for any registry.
type ungated struct{}

func (ungated) permit(string, string, bool) bool    { return true }
func (ungated) program(helper string) string        { return credHelperPrefix + helper }
func (ungated) registryAllowed(string, string) bool { return true }

// allowlistGate gates a job-controlled source against
// allowed_docker_credential_helpers; a gate built from an empty or unset
// allowlist permits no job-supplied helper.
type allowlistGate struct {
	parsed map[string]allowedHelper
	logger Logger
}

func newAllowlistGate(allowlist map[string][]string, logger Logger) (*allowlistGate, error) {
	g := &allowlistGate{parsed: make(map[string]allowedHelper, len(allowlist)), logger: logger}
	for key, patterns := range allowlist {
		helper, program, err := parseAllowlistKey(key)
		if err != nil {
			return nil, err
		}
		if _, dup := g.parsed[helper]; dup {
			return nil, fmt.Errorf("helper %q is configured more than once", helper)
		}
		if len(patterns) == 0 {
			return nil, fmt.Errorf("helper %q must list at least one registry pattern", helper)
		}
		for _, p := range patterns {
			// doublestar.ValidatePattern accepts "", which can never match a registry.
			if p == "" || !doublestar.ValidatePattern(p) {
				return nil, fmt.Errorf("helper %q has an invalid registry pattern %q", helper, p)
			}
		}
		g.parsed[helper] = allowedHelper{key: key, program: program, patterns: patterns}
	}

	return g, nil
}

// ValidateAllowedHelpers returns an error when an allowed_docker_credential_helpers
// map is malformed, so bad keys or registry patterns are caught when the runner
// loads its configuration rather than when a job resolves credentials.
func ValidateAllowedHelpers(allowlist map[string][]string) error {
	_, err := newAllowlistGate(allowlist, nil)
	return err
}

// parseAllowlistKey derives the helper name jobs reference and the program to
// execute from an allowed_docker_credential_helpers key. A bare name resolves
// "docker-credential-<name>" on PATH; an absolute path pins the exact binary,
// and must keep docker's naming convention so the referenced name can be
// derived from it.
func parseAllowlistKey(key string) (helper, program string, err error) {
	base := filepath.Base(key)
	switch {
	case key == "" || key == "." || key == "..":
		return "", "", errors.New("helper name must not be empty")
	case key == base && strings.HasPrefix(key, credHelperPrefix):
		// The operator wrote the binary name, but jobs reference the suffix.
		return "", "", fmt.Errorf("helper %q must be the name jobs reference, %q, not the binary name",
			key, strings.TrimPrefix(key, credHelperPrefix))
	case key == base:
		return key, credHelperPrefix + key, nil
	case !filepath.IsAbs(key):
		return "", "", fmt.Errorf("helper %q must be a bare helper name or an absolute path", key)
	}

	// Jobs reference a helper by the suffix without an executable extension,
	// so a pinned docker-credential-<name>.exe on Windows must derive <name>.
	name, ok := strings.CutPrefix(base, credHelperPrefix)
	if strings.EqualFold(filepath.Ext(name), ".exe") {
		name = name[:len(name)-len(".exe")]
	}
	// filepath.Base drops a trailing separator that exec would still see, so
	// reject it here instead of failing every job with ENOTDIR.
	if !ok || name == "" || os.IsPathSeparator(key[len(key)-1]) {
		return "", "", fmt.Errorf("pinned helper %q must be a binary named %s<helper>", key, credHelperPrefix)
	}
	return name, key, nil
}

// hasUnsafeRegistryChars reports whether a registry name contains any character
// outside the set registry references legitimately use. It is a positive
// allowlist, not a denylist: the credential helper protocol is line-oriented
// over stdin, so whitespace or a control character could smuggle extra input
// into the helper, and "@" (with its Unicode lookalikes) introduces URL userinfo
// that helpers and kubelet parse as a different host. A denylist misses format
// and zero-width runes, so anything outside [A-Za-z0-9._:/[]-] is unsafe.
func hasUnsafeRegistryChars(registry string) bool {
	return strings.ContainsFunc(registry, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return false
		case r == '.', r == '-', r == '_', r == ':', r == '/', r == '[', r == ']':
			return false
		default:
			return true
		}
	})
}

// permit reports whether a credential helper may be executed for registry, and
// logs any it skips: a warning, except for an unlisted helper whose binary is
// not installed, which could never have run and so is logged at debug level.
// isStore marks a credsStore, which is asked about every registry it holds and
// carries no registry here; its listed registries are instead narrowed at
// retrieval by registryAllowed. A skipped helper only drops its own entry, so
// static credentials and permitted helpers in the same DOCKER_AUTH_CONFIG still
// apply.
func (g *allowlistGate) permit(helper, registry string, isStore bool) bool {
	if !isStore {
		// A credHelpers key is the registry the helper is asked about; an empty key
		// would bypass the patterns and an unsafe one (see hasUnsafeRegistryChars) is never run.
		if registry == "" {
			g.logger.Warningln(fmt.Sprintf(
				"Ignoring credential helper %q requested by the job's DOCKER_AUTH_CONFIG: "+
					"the registry name is empty", helper))
			return false
		}
		if hasUnsafeRegistryChars(registry) {
			g.logger.Warningln(fmt.Sprintf(
				"Ignoring credential helper %q requested by the job's DOCKER_AUTH_CONFIG for registry %q: "+
					"the registry name contains characters outside the permitted set", helper, registry))
			return false
		}
	}

	if h, ok := g.parsed[helper]; ok {
		if isStore || matchesAllowedRegistry(h.patterns, registry) {
			return true
		}
		// Listed, but not for this registry: the operator configured the
		// helper, so the mismatch is worth an actionable warning.
		g.logger.Warningln(notAllowedMessage(helper, h.key, registry))
		return false
	}

	// An unlisted helper whose binary is missing never worked, so skip quietly;
	// warn only when withholding a credential that would have resolved. The
	// warning's presence reveals whether docker-credential-<name> is installed,
	// which was already observable before the allowlist and exposes no credential.
	program := g.program(helper)
	if _, err := exec.LookPath(program); err != nil {
		g.logger.Debugln(fmt.Sprintf(
			"Ignoring credential helper %q from DOCKER_AUTH_CONFIG: it is not permitted by "+
				"allowed_docker_credential_helpers and its binary %q is not installed", helper, program))
		return false
	}

	g.logger.Warningln(notAllowedMessage(helper, "", registry))
	return false
}

// notAllowedMessage explains why a credential helper request was skipped and,
// for whoever administers the runner, the exact config.toml change to permit
// it. configKey is the operator's allowlist key for a listed helper, or "" when
// the helper is not listed. The message reaches the job log, which the job
// author reads rather than the runner operator, so it also states the
// operator-free alternatives.
func notAllowedMessage(helper, configKey, registry string) string {
	const remedy = "If you administer this runner, %s under " +
		"[runners.allowed_docker_credential_helpers] in config.toml. Otherwise, use statically " +
		"defined credentials in DOCKER_AUTH_CONFIG, or ask your runner administrator. See %s"

	if configKey != "" {
		// Listed helper (so registry is non-empty); the remedy is to widen its pattern.
		add := fmt.Sprintf("add a pattern matching %q to %q", convertToRegistryPath(registry), configKey)
		return fmt.Sprintf(
			"Ignoring credential helper %q requested by the job's DOCKER_AUTH_CONFIG for registry %q: "+
				"it is permitted, but not for that registry. "+remedy,
			helper, registry, add, credHelperAllowlistDocsURL)
	}

	// A credsStore (empty registry) is asked about all registries.
	target, pattern := "all registries", "<registry pattern>"
	if registry != "" {
		target = fmt.Sprintf("registry %q", registry)
		pattern = convertToRegistryPath(registry)
	}
	add := fmt.Sprintf("allow it by adding %q = [%q]", helper, pattern)
	return fmt.Sprintf(
		"Ignoring credential helper %q requested by the job's DOCKER_AUTH_CONFIG for %s: this runner "+
			"does not permit it. "+remedy,
		helper, target, add, credHelperAllowlistDocsURL)
}

// program returns what to execute for a helper: the operator-pinned path when
// one is configured, or the conventional PATH-resolved name otherwise.
func (g *allowlistGate) program(helper string) string {
	if h, ok := g.parsed[helper]; ok {
		return h.program
	}

	return credHelperPrefix + helper
}

// registryAllowed reports whether a registry listed by a permitted credsStore
// helper may have its credentials retrieved: the registry must be free of unsafe
// characters and match one of the helper's patterns, so an unlisted helper (nil
// patterns) is denied. A skip is logged at debug level, since the name comes
// from the operator-listed helper's own output rather than from the job.
func (g *allowlistGate) registryAllowed(helper, registry string) bool {
	if hasUnsafeRegistryChars(registry) {
		g.logger.Debugln(fmt.Sprintf(
			"Not retrieving credentials for registry %q listed by credential store %q: "+
				"the registry name contains characters outside the permitted set", registry, helper))
		return false
	}
	if !matchesAllowedRegistry(g.parsed[helper].patterns, registry) {
		g.logger.Debugln(fmt.Sprintf(
			"Not retrieving credentials for registry %q listed by credential store %q: "+
				"no allowed pattern for that helper matches it", registry, helper))
		return false
	}
	return true
}

// matchesAllowedRegistry reports whether the registry, normalized with
// convertToRegistryPath, matches any of the glob patterns. Patterns are
// validated when the gate is built, so the match error cannot be a bad pattern.
func matchesAllowedRegistry(patterns []string, registry string) bool {
	registryPath := convertToRegistryPath(registry)
	for _, pattern := range patterns {
		if ok, _ := doublestar.Match(pathWithLowerCaseHostname(pattern), registryPath); ok {
			return true
		}
	}

	return false
}

// ConfigForImage returns the auth configuration for a particular image.
// It gets all configs via [AllConfigs] and returns the one with the longest match for imageName <-> RegistryInfo.RegistryPath
// It returns nil when no matching config can be found.
func (r Resolver) ConfigForImage(
	imageName, dockerAuthConfig, username string,
	credentials []spec.Credentials, logger Logger,
) (*RegistryInfo, error) {
	authConfigs, err := r.AllConfigs(dockerAuthConfig, username, credentials, logger)
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
func (r Resolver) AllConfigs(
	dockerAuthConfig, username string,
	credentials []spec.Credentials, logger Logger,
) (RegistryInfos, error) {
	gate, err := newAllowlistGate(r.jobCredHelperAllowlist, logger)
	if err != nil {
		return nil, fmt.Errorf("allowed_docker_credential_helpers: %w", err)
	}

	resolvers := []func() (string, []types.AuthConfig, error){
		func() (string, []types.AuthConfig, error) {
			return getUserConfiguration(dockerAuthConfig, gate)
		},
		func() (string, []types.AuthConfig, error) {
			return r.getHomeDirConfiguration(username)
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
		if err != nil {
			logger.Warningln(fmt.Sprintf(
				"Failed to resolve credentials from %v: %v. Credentials from this source will not be used.",
				source, err,
			))
			continue
		}

		if len(configs) == 0 {
			continue
		}

		hostnames := []string{} // used only for logging

		for _, conf := range configs {
			registryPath := convertToRegistryPath(conf.ServerAddress)
			hostnames = append(hostnames, registryPath)

			newRegistryInfo := RegistryInfo{
				Path:       registryPath,
				Source:     source,
				AuthConfig: conf,
			}

			if err := res.Append(newRegistryInfo); err != nil {
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

func getUserConfiguration(dockerAuthConfig string, gate credHelperGate) (string, []types.AuthConfig, error) {
	authConfigs, err := readConfigsFromReader(bytes.NewBufferString(dockerAuthConfig), gate)
	if err != nil {
		return configSourceNameUserVariable, nil, err
	}
	if authConfigs == nil {
		return "", nil, nil
	}

	return configSourceNameUserVariable, authConfigs, nil
}

func (r Resolver) getHomeDirConfiguration(username string) (string, []types.AuthConfig, error) {
	sourceFile, authConfigs, err := r.readDockerConfigsFromHomeDir(username)
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

func getBuildConfiguration(credentials []spec.Credentials) (string, []types.AuthConfig, error) {
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

	return configSourceNameJobPayload, authConfigs, nil
}

// normalizeImageRef takes a raw image reference and normalizes it:
//   - cuts off the tag and digest
//   - normalizes docker.io image refs (nginx -> docker.io/nginx, index.docker.io/nginx -> docker.io/nginx)
//   - lower-cases the hostname
func normalizeImageRef(imageName string) string {
	// foo.bar.tld/blipo/blupp:latest -> [ foo.bar.tld/blipp/, blupp:latest ]
	dir, image := path.Split(imageName)

	// remove digest and tag: blupp:latest@sha256:... -> blupp
	image, _, _ = strings.Cut(image, "@")
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
func (r Resolver) readDockerConfigsFromHomeDir(userName string) (string, []types.AuthConfig, error) {
	homeDir := r.homeDir.Get()

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

	// The home directory configuration is owned by the operator of the
	// runner host, so credential helpers it references are not gated.
	authConfigs, err := readConfigsFromReader(f, ungated{})
	return f.Name(), authConfigs, err
}

func readConfigsFromReader(r io.Reader, gate credHelperGate) ([]types.AuthConfig, error) {
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
		authsFromCredentialsStore, err := readConfigsFromCredentialsStore(config, gate)
		if err != nil {
			return nil, err
		}
		maps.Copy(auths, authsFromCredentialsStore)
	}

	if config.CredentialHelpers != nil {
		authsFromCredentialsHelpers, err := readConfigsFromCredentialsHelper(config, gate)
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

func readConfigsFromCredentialsStore(
	config *configfile.ConfigFile, gate credHelperGate,
) (map[string]types.AuthConfig, error) {
	helper := config.CredentialsStore
	if helper != filepath.Base(helper) {
		// Fail processing if credential store attempting path traversal are detected
		return nil, errPathTraversal
	}

	if !gate.permit(helper, "", true) {
		return nil, nil
	}

	program := client.NewShellProgramFunc(gate.program(helper))

	listed, err := client.List(program)
	if err != nil {
		return nil, err
	}

	auths := make(map[string]types.AuthConfig, len(listed))
	for registry := range listed {
		// Only retrieve secrets for registries the helper is allowed to serve;
		// the list step above exposes only server URLs and usernames. Those names
		// come from the operator-listed helper itself, not the job, so they are
		// checked for unsafe characters and gated by pattern, logging any skip.
		if !gate.registryAllowed(helper, registry) {
			continue
		}

		auth, err := getCredentialsFromProgram(program, registry)
		if err != nil {
			return nil, err
		}
		auths[registry] = auth
	}

	return auths, nil
}

func readConfigsFromCredentialsHelper(
	config *configfile.ConfigFile, gate credHelperGate,
) (map[string]types.AuthConfig, error) {
	helpersAuths := make(map[string]types.AuthConfig)

	for registry, helper := range config.CredentialHelpers {
		if helper != filepath.Base(helper) {
			// Fail processing if credential helpers attempting path traversal are detected
			return nil, errPathTraversal
		}

		if !gate.permit(helper, registry, false) {
			continue
		}

		newAuths, err := getCredentialsFromProgram(client.NewShellProgramFunc(gate.program(helper)), registry)
		if err != nil {
			return nil, err
		}

		helpersAuths[registry] = newAuths
	}

	return helpersAuths, nil
}

// getCredentialsFromProgram executes the helper's "get" action for
// serverAddress, keeping docker/cli's native store semantics for the answer: a
// not-found response is an empty config rather than an error, and the "<token>"
// username marks the secret as an identity token. Unlike the native store, the
// result is not merged with the static auths entry for the same key, so a
// static RegistryToken alongside a helper entry is not carried over. Callers
// store the result under the registry key, replacing any static entry there,
// including with the empty config on not-found.
func getCredentialsFromProgram(program client.ProgramFunc, serverAddress string) (types.AuthConfig, error) {
	var ret types.AuthConfig

	creds, err := client.Get(program, serverAddress)
	if err != nil {
		if helpercredentials.IsErrCredentialsNotFound(err) {
			return ret, nil
		}
		return ret, err
	}

	if creds.Username == tokenUsername {
		ret.IdentityToken = creds.Secret
	} else {
		ret.Username = creds.Username
		ret.Password = creds.Secret
	}
	ret.ServerAddress = serverAddress

	return ret, nil
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
