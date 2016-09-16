// +build darwin dragonfly freebsd linux netbsd openbsd

package shell

func RewriteBashPath(path string) string {
	return path
}
