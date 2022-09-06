//go:build !windows

package parser

import (
	gopath "path"
	"regexp"
	"strings"
)

// This is an implementation of helpers/path.Path interface for Windows that
// is designed for sole-use by docker's volume parser.
//
// Dealing with Windows path operations typically requires the code to be run
// from a windows host. However, if you know how the path is ultimately used and
// checked, approximations are typically fine.
type windowsPath struct {
}

// windowsNamedPipesExp matches a named pipe path (starts with `\\.\pipe\`, possibly with / instead of \)
var windowsNamedPipeRe = regexp.MustCompile(`(?i)^[/\\]{2}\.[/\\]pipe[/\\][^:*?"<>|\r\n]+$`)

// Join joins path elements with \. This version of Join is not cleaned, so:
// Join(C:\windows\a/b, ../c\d) will return: C:\windows\a/b\../c\d
func (p *windowsPath) Join(elem ...string) string {
	return strings.Join(elem, "\\")
}

// IsAbs returns whether the path provided is an absolute path.
func (p *windowsPath) IsAbs(path string) bool {
	if windowsNamedPipeRe.MatchString(path) {
		return true
	}

	// https://docs.microsoft.com/en-gb/windows/win32/fileio/naming-a-file#fully-qualified-vs-relative-paths
	switch {
	// \absolute.txt, /absolute.txt, \absolute.txt, //absolute.txt
	case strings.HasPrefix(path, "\\") || strings.HasPrefix(path, "/"):
		return true

	case len(path) > 1 && path[1] == ':': // c:\, d:/ etc.
		return true
	}

	return false
}

func (p *windowsPath) IsRoot(path string) bool {
	if windowsNamedPipeRe.MatchString(path) {
		return false
	}

	if !p.IsAbs(path) {
		return false
	}

	if strings.HasPrefix(path, "\\\\") || strings.HasPrefix(path, "//") {
		return strings.HasSuffix(path, "\\") || strings.HasSuffix(path, "/")
	}

	path = p.convert(path, false)

	return len(path) > 1 && strings.Count(path, "/") < 2
}

func (p *windowsPath) Contains(basePath, targetPath string) bool {
	return strings.HasPrefix(p.convert(targetPath, true), p.convert(basePath, true))
}

// convert absolute path to a regular and absolute forward-slash based path,
// useful only for comparisons.
//
// c:\hello\world -> /c/hello/world/
// \\server_name/hello -> /server_name/hello/
func (p *windowsPath) convert(pathname string, dir bool) string {
	if len(pathname) > 1 && pathname[1] == ':' {
		pathname = pathname[:1] + pathname[2:]
	}
	pathname = strings.NewReplacer("\\", "/", ":", "/").Replace(pathname)
	pathname = gopath.Clean("/" + pathname)

	if dir && !strings.HasSuffix(pathname, "/") {
		return pathname + "/"
	}
	return pathname
}

func newWindowsPath() *windowsPath {
	return &windowsPath{}
}
