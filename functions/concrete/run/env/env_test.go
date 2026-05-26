//go:build !integration

package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandValue_GitLabEnvTakesPriorityOverEnv(t *testing.T) {
	e := &Env{
		Env:       map[string]string{"MY_VAR": "static"},
		GitLabEnv: map[string]string{"MY_VAR": "dynamic"},
	}

	assert.Equal(t, "dynamic", e.ExpandValue("$MY_VAR"),
		"GitLabEnv overlay must shadow Env for the same key")
}

// TestExpandValue_ShellEscapes covers `$$` escape and shell special
// parameter handling, per
// https://docs.gitlab.com/ci/variables/#use-variables-in-other-variables.
func TestExpandValue_ShellEscapes(t *testing.T) {
	e := &Env{Env: map[string]string{
		// Names matching shell special params must resolve to empty,
		// not leak the same-keyed env entry.
		"$": "should-not-leak-dollar",
		"*": "should-not-leak-star",
		"0": "should-not-leak-zero",
		"?": "should-not-leak-qm",
	}}

	cases := map[string]struct {
		in   string
		want string
	}{
		"$$ expands to literal $":                     {"$$", "$"},
		"$$ adjacent to text preserves $":             {"PRICE=$$100", "PRICE=$100"},
		"$$ inside ${} braces":                        {"${PATH:-$$}", ""}, // ${...} with default not supported; just guard no panic + no leak
		"$* resolves to empty":                        {"$*", ""},
		"$0 resolves to empty":                        {"$0", ""},
		"$? resolves to empty":                        {"$?", ""},
		"unknown var resolves to empty":               {"$NO_SUCH_VAR", ""},
		"plain literal passes through":                {"literal text", "literal text"},
		"empty string in -> empty string out":         {"", ""},
		"escape followed by var leaves var intact":    {"$$VAR_NAME", "$VAR_NAME"},
		"escape inside ${} should not eat the braces": {"$${BRACED}", "${BRACED}"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, e.ExpandValue(tc.in))
		})
	}
}
