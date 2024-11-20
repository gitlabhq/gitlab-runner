//go:build !integration

package homedir

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {
	tests := map[string]string{
		"windows": "USERPROFILE",
		"plan9":   "home",
		"linux":   "HOME",
		"random":  "HOME",
	}

	for os, expectedVarName := range tests {
		t.Run(os, func(t *testing.T) {
			hd := HomeDir{os: os}

			assert.Equal(t, expectedVarName, hd.Env())
		})
	}
}

func TestGetWDOrEmpty(t *testing.T) {
	tests := map[string]struct {
		wdDir      string
		wdErr      error
		expectedWd string
	}{
		"default": {
			wdDir:      "/some/dir",
			expectedWd: "/some/dir",
		},
		"empty working dir": {
			wdDir:      "",
			expectedWd: "",
		},
		"WorkingDirectory returns error": {
			wdDir:      "not-used",
			wdErr:      assert.AnError,
			expectedWd: "",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			hd := HomeDir{
				workingDirectory: func() (string, error) {
					return test.wdDir, test.wdErr
				},
			}

			assert.Equal(t, test.expectedWd, hd.GetWDOrEmpty())
		})
	}
}

func TestGet(t *testing.T) {
	tests := map[string]struct {
		os               string
		userHomeDir      func() (string, error)
		currenUser       func() (*user.User, error)
		expectedVarValue string
	}{
		"userHomeDir returns dir": {
			userHomeDir:      func() (string, error) { return "/some/dir", nil },
			expectedVarValue: "/some/dir",
		},
		"userHomeDir returns no dir but currentUser does": {
			userHomeDir: func() (string, error) { return "", assert.AnError },
			currenUser: func() (*user.User, error) {
				return &user.User{HomeDir: "/some/user/home/dir"}, nil
			},
			expectedVarValue: "/some/user/home/dir",
		},
		"userHomeDir returns no dir and currentUser errors": {
			userHomeDir:      func() (string, error) { return "", assert.AnError },
			currenUser:       func() (*user.User, error) { return nil, assert.AnError },
			expectedVarValue: "",
		},
		"userHomeDir returns no dir on windows": {
			os:               "windows",
			userHomeDir:      func() (string, error) { return "", assert.AnError },
			expectedVarValue: "",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			hd := HomeDir{
				os:          test.os,
				currentUser: test.currenUser,
				userHomeDir: test.userHomeDir,
			}

			assert.Equal(t, test.expectedVarValue, hd.Get())
		})
	}
}

func TestFix(t *testing.T) {
	tests := map[string]struct {
		os                 string
		homeEnvVarVal      string
		userHomeDir        func() (string, error)
		currenUser         func() (*user.User, error)
		expectedErr        error
		expectedHomeEnvVal string
	}{
		"home from env": {
			homeEnvVarVal:      "/some/home/dir",
			expectedHomeEnvVal: "/some/home/dir",
		},
		"home not set but userHomeDir returns home dir": {
			userHomeDir:        func() (string, error) { return "/some/user/home/dir", nil },
			expectedHomeEnvVal: "/some/user/home/dir",
		},
		"home not set and userHomeDir returns no home dir": {
			userHomeDir: func() (string, error) { return "", assert.AnError },
			currenUser: func() (*user.User, error) {
				return &user.User{HomeDir: "/home/dir/from/current/user"}, nil
			},
			expectedHomeEnvVal: "/home/dir/from/current/user",
		},
		"home not set and userHomeDir returns no home dir and currentUser returns no home dir": {
			userHomeDir: func() (string, error) { return "", assert.AnError },
			currenUser:  func() (*user.User, error) { return nil, assert.AnError },
			expectedErr: ErrHomedirVariableNotSet,
		},
		"home not set and userHomeDir returns no home dir on windows": {
			os:          "windows",
			userHomeDir: func() (string, error) { return "", assert.AnError },
			expectedErr: ErrHomedirVariableNotSet,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			fakeEnv := fakeEnv{}

			hd := HomeDir{
				setEnv:      fakeEnv.Set,
				getEnv:      fakeEnv.Get,
				os:          test.os,
				currentUser: test.currenUser,
				userHomeDir: test.userHomeDir,
			}

			homeEnvVarName := hd.Env()
			if test.homeEnvVarVal != "" {
				_ = fakeEnv.Set(homeEnvVarName, test.homeEnvVarVal)
			} else {
				fakeEnv.Unset(homeEnvVarName)
			}

			err := hd.Fix()

			if test.expectedErr != nil {
				assert.ErrorIs(t, err, test.expectedErr)
				return
			}

			if assert.NoError(t, err) {
				assert.Equal(t, test.expectedHomeEnvVal, fakeEnv.Get(homeEnvVarName),
					"expected the env var %q to be set to %q", homeEnvVarName, test.expectedHomeEnvVal,
				)
			}
		})
	}
}

type fakeEnv map[string]string

func (f fakeEnv) Get(k string) string {
	return f[k]
}

func (f fakeEnv) Set(k, v string) error {
	f[k] = v
	return nil
}

func (f fakeEnv) Unset(k string) {
	delete(f, k)
}
