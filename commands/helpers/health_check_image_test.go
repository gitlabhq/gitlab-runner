//go:build !integration

package helpers

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExposedPorts(t *testing.T) {
	ports := parseExposedPorts(map[string]struct{}{
		"6379/tcp": {},
		"53/udp":   {},
		"8080":     {},
		"443/TCP":  {},
		"bogus":    {},
		"70000":    {},
	})
	assert.Equal(t, []string{"443", "6379", "8080"}, ports)
	assert.Empty(t, parseExposedPorts(nil))

	many := map[string]struct{}{}
	for p := 1000; p < 1030; p++ {
		many[strconv.Itoa(p)+"/tcp"] = struct{}{}
	}
	capped := parseExposedPorts(many)
	assert.Len(t, capped, maxPortsCheck)
	assert.Equal(t, "1000", capped[0])
	assert.Equal(t, "1019", capped[len(capped)-1])
}

func TestLoadRegistryKeychain(t *testing.T) {
	dir := t.TempDir()
	write := func(sub, file, content string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, sub, file), []byte(content), 0o600))
	}
	write("0", ".dockercfg",
		`{"https://index.docker.io/v1/": {"auth": "aHViOnB3"}, "registry.example.com": {"username": "u", "password": "p"}}`)
	write("1", ".dockerconfigjson", `{"auths": {
		"ghcr.io": {"auth": "Z2g6dG9r"},
		"https://registry.example.com/team-a": {"username": "a", "password": "pa"},
		"registry.example.com/v2/team-b/": {"username": "b", "password": "pb"},
		"registry.example.com": {"username": "u2", "password": "p2"}
	}}`)
	write("2", ".dockerconfigjson", `{"auths": {}, "credsStore": "desktop"}`)
	write("3", ".dockercfg", `{`)
	write("..data", ".dockercfg", `{"quay.io": {"username": "dup", "password": "dup"}}`)

	kc := loadRegistryKeychain(dir)

	creds := func(ref string) []string {
		parsed, err := name.ParseReference(ref)
		require.NoError(t, err)
		var out []string
		for _, auth := range kc.candidates(parsed.Context()) {
			if auth == authn.Anonymous {
				out = append(out, "anonymous")
				continue
			}
			got, err := auth.Authorization()
			require.NoError(t, err)
			out = append(out, got.Username+":"+got.Password)
		}
		return out
	}

	assert.Equal(t, []string{"hub:pw"}, creds("redis:7"))
	assert.Equal(t, []string{"u:p", "u2:p2"}, creds("registry.example.com/img:1"), "both secrets for one registry, in mount order")
	assert.Equal(t, []string{"a:pa", "u:p", "u2:p2"}, creds("registry.example.com/team-a/img:1"), "scheme key keeps its path")
	assert.Equal(t, []string{"b:pb", "u:p", "u2:p2"}, creds("registry.example.com/team-b/img:1"), "/v2/ prefix is removed")
	assert.Equal(t, []string{"gh:tok"}, creds("ghcr.io/org/img@sha256:0000000000000000000000000000000000000000000000000000000000000000"))
	assert.Equal(t, []string{"anonymous"}, creds("quay.io/img:1"), "..data is not read")
}

func TestLoadRegistryKeychainEmpty(t *testing.T) {
	assert.Empty(t, loadRegistryKeychain("").auths)
	assert.Empty(t, loadRegistryKeychain(filepath.Join(t.TempDir(), "missing")).auths)
}

func TestNormalizeRegistryPath(t *testing.T) {
	tests := map[string]string{
		"https://index.docker.io/v1/":          "index.docker.io",
		"docker.io":                            "index.docker.io",
		"registry-1.docker.io/library":         "index.docker.io/library",
		"https://registry.example.com/team-a":  "registry.example.com/team-a",
		"registry.example.com/v2/team-b/":      "registry.example.com/team-b",
		"registry.example.com/v1":              "registry.example.com",
		"REGISTRY.example.com:5000/a/b":        "registry.example.com:5000/a/b",
		"http://registry.example.com:5000/v2/": "registry.example.com:5000",
	}
	for key, want := range tests {
		assert.Equal(t, want, normalizeRegistryPath(key), key)
	}
}

func TestReferenceCandidates(t *testing.T) {
	const digest = "@sha256:0000000000000000000000000000000000000000000000000000000000000000"
	tests := map[string][]string{
		"redis:7":          {"redis:7"},
		"redis:7" + digest: {"redis:7" + digest, "redis:7"},
		"redis" + digest:   {"redis" + digest},
		"registry.example.com:5000/a/b:1" + digest: {"registry.example.com:5000/a/b:1" + digest, "registry.example.com:5000/a/b:1"},
		"registry.example.com:5000/a/b" + digest:   {"registry.example.com:5000/a/b" + digest},
	}
	for ref, want := range tests {
		assert.Equal(t, want, referenceCandidates(ref), ref)
	}
}
