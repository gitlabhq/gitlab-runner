package helpers

import "strings"

func ToBackslash(path string) string {
	return strings.ReplaceAll(path, "/", "\\")
}

func ToSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}
