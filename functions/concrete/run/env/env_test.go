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
