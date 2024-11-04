//go:build !integration

package url_helpers

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemovingAllSensitiveData(t *testing.T) {
	url := CleanURL("https://user:password@gitlab.com/gitlab?key=value#fragment")
	assert.Equal(t, "https://gitlab.com/gitlab", url)
}

func TestInvalidURL(t *testing.T) {
	assert.Empty(t, CleanURL("://invalid URL"))
}

func TestOnlySchemeAndHost(t *testing.T) {
	tests := map[string]string{
		"":                                 "",
		"https://gitlab.com":               "https://gitlab.com",
		"https://gitlab.com/":              "https://gitlab.com",
		"https://gitlab.com/some/path":     "https://gitlab.com",
		"https://gitlab.com#foo":           "https://gitlab.com",
		"https://gitlab.com/blipp#foo":     "https://gitlab.com",
		"https://gitlab.com?foo&bar=baz":   "https://gitlab.com",
		"https://user@gitlab.com":          "https://gitlab.com",
		"https://user:password@gitlab.com": "https://gitlab.com",
		"ssh://git@gitlab.com":             "ssh://gitlab.com",
		"git://gitlab.com:444":             "git://gitlab.com:444",
		"http://10.0.0.1:345#blupp":        "http://10.0.0.1:345",
		"blipp://localhost:123/test":       "blipp://localhost:123",
	}

	for inputURL, expectedURL := range tests {
		t.Run(inputURL, func(t *testing.T) {
			orgURL, err := url.Parse(inputURL)
			require.NoError(t, err, "parsing input URL")

			newURL := OnlySchemeAndHost(orgURL)
			assert.Equal(t, expectedURL, newURL.String())
		})
	}
}
