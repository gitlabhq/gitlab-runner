package release

import (
	"gitlab.com/gitlab-org/ci-cd/runner-tools/release-index-generator/fs"
)

type Info struct {
	directoryScanner fs.Scanner

	Name      string
	Project   string
	SourceURL string
	Ref       string
	Revision  string
	CreatedAt string
	Files     []fs.FileEntry
}

func (ri *Info) prepareFiles() error {
	ri.Files = make([]fs.FileEntry, 0)

	return ri.directoryScanner.Walk(func(entry fs.FileEntry) error {
		if entry.FileName != indexFileName {
			ri.Files = append(ri.Files, entry)
		}

		return nil
	})
}
