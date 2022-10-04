package parser

//go:generate mockery --name=Parser --inpackage
type Parser interface {
	ParseVolume(spec string) (*Volume, error)
	Path() Path
}

type Path interface {
	Join(elem ...string) string
	IsAbs(path string) bool
	IsRoot(path string) bool
	Contains(basePath, targetPath string) bool
}
