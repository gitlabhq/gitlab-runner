package test

import (
	"os"
	"runtime"
	"testing"
)

const (
	OSWindows = "windows"
	OSLinux   = "linux"
)

func SkipIfGitLabCI(t *testing.T) {
	_, ok := os.LookupEnv("CI")
	if ok {
		t.Skipf("Skipping test on CI builds: %s", t.Name())
	}
}

func SkipIfGitLabCIOn(t *testing.T, os string) {
	if runtime.GOOS != os {
		return
	}

	SkipIfGitLabCI(t)
}

func SkipIfGitLabCIWithMessage(t *testing.T, msg string) {
	_, ok := os.LookupEnv("CI")
	if ok {
		t.Skipf("Skipping test on CI builds: %s - %s", t.Name(), msg)
	}
}
