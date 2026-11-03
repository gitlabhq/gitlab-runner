package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/sirupsen/logrus"
)

const (
	// For all images of a job together.
	imageInspectTimeout = 45 * time.Second

	// Same cap as the Docker executor.
	maxPortsCheck = 20
)

var registryAuthFiles = []string{".dockerconfigjson", ".dockercfg"}

// Same node as the services, so the helper's own platform is the right one.
// A "repo:tag@digest" reference is tried at the digest first and at the tag
// when the registry does not know that digest.
func imageExposedPorts(ctx context.Context, ref string, keychain *registryKeychain) ([]string, error) {
	var lastErr error
	for _, candidate := range referenceCandidates(ref) {
		parsed, err := name.ParseReference(candidate)
		if err != nil {
			return nil, fmt.Errorf("parsing image reference %q: %w", candidate, err)
		}
		ports, err := fetchWithCredentials(ctx, parsed, keychain)
		if err == nil {
			return ports, nil
		}
		lastErr = err
		if !isNotFound(err) {
			break
		}
	}
	return nil, lastErr
}

func referenceCandidates(ref string) []string {
	base, _, hasDigest := strings.Cut(ref, "@")
	if !hasDigest || !strings.Contains(base[strings.LastIndex(base, "/")+1:], ":") {
		return []string{ref}
	}
	return []string{ref, base}
}

func fetchWithCredentials(ctx context.Context, ref name.Reference, keychain *registryKeychain) ([]string, error) {
	var lastErr error
	for _, auth := range keychain.candidates(ref.Context()) {
		ports, err := fetchExposedPorts(ctx, ref, auth)
		if err == nil {
			return ports, nil
		}
		lastErr = err
		if !isAuthError(err) {
			break
		}
	}
	return nil, lastErr
}

func fetchExposedPorts(ctx context.Context, ref name.Reference, auth authn.Authenticator) ([]string, error) {
	opts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuth(auth),
		remote.WithPlatform(v1.Platform{OS: runtime.GOOS, Architecture: runtime.GOARCH}),
	}

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	img, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("resolving image for %s/%s: %w", runtime.GOOS, runtime.GOARCH, err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("fetching image config: %w", err)
	}
	return parseExposedPorts(cfg.Config.ExposedPorts), nil
}

func isAuthError(err error) bool {
	if terr, ok := errors.AsType[*transport.Error](err); ok {
		return terr.StatusCode == http.StatusUnauthorized || terr.StatusCode == http.StatusForbidden
	}
	return false
}

func isNotFound(err error) bool {
	if terr, ok := errors.AsType[*transport.Error](err); ok {
		return terr.StatusCode == http.StatusNotFound
	}
	return false
}

// Keys look like "6379/tcp", "53/udp" or "8080".
func parseExposedPorts(exposed map[string]struct{}) []string {
	var ports []int
	for spec := range exposed {
		portStr, proto, _ := strings.Cut(spec, "/")
		if proto != "" && !strings.EqualFold(proto, "tcp") {
			continue
		}
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			continue
		}
		ports = append(ports, int(port))
	}
	sort.Ints(ports)
	if len(ports) > maxPortsCheck {
		ports = ports[:maxPortsCheck]
	}

	result := make([]string, 0, len(ports))
	for _, p := range ports {
		result = append(result, strconv.Itoa(p))
	}
	return result
}

type registryKeychain struct {
	auths map[string][]authn.AuthConfig
}

func loadRegistryKeychain(dir string) *registryKeychain {
	kc := &registryKeychain{auths: map[string][]authn.AuthConfig{}}
	if dir == "" {
		return kc
	}

	for _, file := range findRegistryAuthFiles(dir) {
		data, err := os.ReadFile(file)
		if err != nil {
			logrus.Warningln(fmt.Sprintf("Skipping registry credentials %s: %v", file, err))
			continue
		}
		auths, err := parseDockerConfig(data)
		if err != nil {
			logrus.Warningln(fmt.Sprintf("Skipping registry credentials %s: %v", file, err))
			continue
		}
		for key, cfg := range auths {
			path := normalizeRegistryPath(key)
			kc.auths[path] = append(kc.auths[path], cfg)
		}
	}
	return kc
}

// One subdirectory per mounted secret, skipping the kubelet's ..data symlinks.
func findRegistryAuthFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), "..") {
			continue
		}
		for _, f := range registryAuthFiles {
			path := filepath.Join(dir, entry.Name(), f)
			if _, err := os.Stat(path); err == nil {
				files = append(files, path)
			}
		}
	}
	return files
}

func parseDockerConfig(data []byte) (map[string]authn.AuthConfig, error) {
	var wrapped struct {
		Auths map[string]authn.AuthConfig `json:"auths"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Auths) > 0 {
		return wrapped.Auths, nil
	}

	// .dockercfg format, skipping non-object values such as credsStore.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	auths := map[string]authn.AuthConfig{}
	for key, value := range raw {
		var cfg authn.AuthConfig
		if err := json.Unmarshal(value, &cfg); err == nil {
			auths[key] = cfg
		}
	}
	return auths, nil
}

// Same rules as the kubelet: drop the scheme and a leading /v1/ or /v2/, keep
// any repository path.
func normalizeRegistryPath(key string) string {
	for _, scheme := range []string{"https://", "http://"} {
		key = strings.TrimPrefix(key, scheme)
	}
	key = strings.TrimSuffix(key, "/")

	registry, repository, _ := strings.Cut(key, "/")
	registry = strings.ToLower(registry)
	switch registry {
	case "docker.io", "index.docker.io", "registry-1.docker.io", "registry.hub.docker.com":
		registry = name.DefaultRegistry
	}
	for _, version := range []string{"v1", "v2"} {
		if repository == version {
			repository = ""
		}
		repository = strings.TrimPrefix(repository, version+"/")
	}
	if repository == "" {
		return registry
	}
	return registry + "/" + repository
}

func parentPath(path string) string {
	index := strings.LastIndex(path, "/")
	if index == -1 {
		return ""
	}
	return path[:index]
}

// Most specific key first, every secret for it, anonymous when nothing matches.
func (kc *registryKeychain) candidates(target authn.Resource) []authn.Authenticator {
	var auths []authn.Authenticator
	for path := normalizeRegistryPath(target.String()); path != ""; path = parentPath(path) {
		for _, cfg := range kc.auths[path] {
			auths = append(auths, authn.FromConfig(cfg))
		}
	}
	if len(auths) == 0 {
		return []authn.Authenticator{authn.Anonymous}
	}
	return auths
}
