package path

import "path"

type unixPath struct{}

func (p *unixPath) Join(elem ...string) string {
	return path.Join(elem...)
}

func (p *unixPath) IsAbs(pathname string) bool {
	return path.IsAbs(pathname)
}

func (p *unixPath) IsRoot(pathname string) bool {
	pathname = path.Clean(pathname)
	return path.IsAbs(pathname) && path.Dir(pathname) == pathname
}

func (p *unixPath) Contains(basePath, targetPath string) bool {
	basePath = path.Clean(basePath)
	targetPath = path.Clean(targetPath)

	for {
		if targetPath == basePath {
			return true
		}
		if p.IsRoot(targetPath) || targetPath == "." {
			return false
		}
		targetPath = path.Dir(targetPath)
	}
}

//revive:disable:unexported-return
func NewUnixPath() *unixPath {
	return &unixPath{}
}
