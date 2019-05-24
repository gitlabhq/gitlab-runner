package path

// Path is used for manipulation/checks of a path depending on the OS.
// Each supported OS needs to have its own implementation.
type Path interface {
	Join(elem ...string) string
	IsAbs(path string) bool
	IsRoot(path string) bool
	Contains(basePath, targetPath string) bool
}
