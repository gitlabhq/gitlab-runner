package fs

type FileEntry struct {
	FileName     string
	FullPath     string
	RelativePath string
	Checksum     string
	SizeMb       float64
}
