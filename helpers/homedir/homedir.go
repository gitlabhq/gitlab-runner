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

type HomeDir struct {
	os               string
	workingDirectory func() (string, error)
	currentUser      func() (*user.User, error)
	userHomeDir      func() (string, error)
	getEnv           func(string) string
	setEnv           func(string, string) error
}

func New() HomeDir {
	return HomeDir{
		os:               runtime.GOOS,
		workingDirectory: os.Getwd,
		currentUser:      user.Current,
		userHomeDir:      os.UserHomeDir,
		getEnv:           os.Getenv,
		setEnv:           os.Setenv,
	}
}

func (hd HomeDir) GetWDOrEmpty() string {
	dir, err := hd.workingDirectory()
	if err == nil {
		return dir
	}
	return ""
}

// Env returns the name of environment variable storing the current user's
// home directory path. Depending on the current platform.
func (hd HomeDir) Env() string {
	switch hd.os {
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
func (hd HomeDir) Get() string {
	home, _ := hd.userHomeDir()
	if home == "" && hd.os != "windows" {
		if u, err := hd.currentUser(); err == nil {
			return u.HomeDir
		}
	}
	return home
}

// Fix tries to set the expected home directory environment variable
// to the detected current user's home directory, if it's not already
// present.
//
// If the variable isn't present, and we can't detect current user's home
// directory, the ErrHomedirVariableNotSet error is returned.
func (hd HomeDir) Fix() error {
	env := hd.Env()
	if hd.getEnv(env) != "" {
		return nil
	}

	homedir := hd.Get()
	if homedir == "" {
		return fmt.Errorf("%w: %q", ErrHomedirVariableNotSet, env)
	}

	_ = hd.setEnv(env, homedir)

	return nil
}
