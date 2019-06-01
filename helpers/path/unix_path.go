package path

import golang_path "path"

type unixPath struct{}

func (p *unixPath) Join(elem ...string) string {
	return golang_path.Join(elem...)
}

func (p *unixPath) IsAbs(path string) bool {
	path = golang_path.Clean(path)
	return golang_path.IsAbs(path)
}

func (p *unixPath) IsRoot(path string) bool {
	path = golang_path.Clean(path)
	return golang_path.IsAbs(path) && golang_path.Dir(path) == path
}

func (p *unixPath) Contains(basePath, targetPath string) bool {
	basePath = golang_path.Clean(basePath)
	targetPath = golang_path.Clean(targetPath)

	for {
		if targetPath == basePath {
			return true
		}
		if p.IsRoot(targetPath) || targetPath == "." {
			return false
		}
		targetPath = golang_path.Dir(targetPath)
	}
}

func NewUnixPath() Path {
	return &unixPath{}
}
