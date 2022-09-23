//go:build windows

// This implementation only works when compiled for Windows
// as this uses the `path/filepath` which is platform dependent

package path

import (
	"path/filepath"
	"regexp"
	"strings"
)

type windowsPath struct {
}

// windowsNamedPipesExp matches a named pipe path (starts with `\\.\pipe\`, possibly with / instead of \)
var windowsNamedPipe = regexp.MustCompile(`(?i)^[/\\]{2}\.[/\\]pipe[/\\][^:*?"<>|\r\n]+$`)

func (p *windowsPath) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (p *windowsPath) IsAbs(path string) bool {
	if windowsNamedPipe.MatchString(path) {
		return true
	}

	path = filepath.Clean(path)
	return filepath.IsAbs(path)
}

func (p *windowsPath) IsRoot(path string) bool {
	if windowsNamedPipe.MatchString(path) {
		return false
	}

	path = filepath.Clean(path)
	return filepath.IsAbs(path) && filepath.Dir(path) == path
}

func (p *windowsPath) Contains(basePath, targetPath string) bool {
	// we use `filepath.Rel` as this perform OS-specific comparison
	// and this set of functions is compiled using OS-specific golang filepath
	relativePath, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}

	// if it starts with `..` it tries to escape the path
	if strings.HasPrefix(relativePath, "..") {
		return false
	}

	return true
}

//revive:disable:unexported-return
func NewWindowsPath() *windowsPath {
	return &windowsPath{}
}
