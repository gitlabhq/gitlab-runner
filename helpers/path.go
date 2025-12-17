package helpers

import (
	"path/filepath"
	"strings"
)

func ToBackslash(path string) string {
	return strings.ReplaceAll(path, "/", "\\")
}

func ToSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

// IsImmediateChild checks if child is an immediate subdirectory of parent.
// Both paths are cleaned and converted to absolute paths before comparison.
// If it is not able to determine the relative path between parent and child, returns false.
func IsImmediateChild(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !strings.Contains(rel, string(filepath.Separator))
}
