package homedir

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
)

var (
	ErrHomedirVariableNotSet = fmt.Errorf("homedir variable is not set")
)

func GetWDOrEmpty() string {
	dir, err := os.Getwd()
	if err == nil {
		return dir
	}

	return ""
}

// Env returns the name of environment variable storing the current user's
// home directory path. Depending on the current platform.
func Env() string {
	switch runtime.GOOS {
	case "windows":
		return "USERPROFILE"
	case "plan9":
		return "home"
	default:
		return "HOME"
	}
}

// Get returns the path to the current user's home directory
// given its best effort to detect that.
//
// Implementation copied from https://github.com/docker/docker/blob/v25.0.6/pkg/homedir/homedir.go
//
// Original code was released under Apache 2.0 license and authored
// by the Docker project contributors.
// As the original source deprecated some parts of the code we've been
// relying on, we've decided to copy this small and simple part directly
// to our codebase, leaving track of its origins.
func Get() string {
	home, _ := os.UserHomeDir()
	if home == "" && runtime.GOOS != "windows" {
		if u, err := user.Current(); err == nil {
			return u.HomeDir
		}
	}

	return home
}

var (
	envGetter     = Env
	homedirGetter = Get
)

// Fix tries to set the expected home directory environment variable
// to the detected current user's home directory, if it's not already
// present.
//
// If the variable isn't present, and we can't detect current user's home
// directory, the ErrHomedirVariableNotSet error is returned.
func Fix() error {
	env := envGetter()
	if os.Getenv(env) != "" {
		return nil
	}

	homedir := homedirGetter()
	if homedir == "" {
		return fmt.Errorf("%w: %q", ErrHomedirVariableNotSet, env)
	}

	_ = os.Setenv(env, homedir)

	return nil
}
