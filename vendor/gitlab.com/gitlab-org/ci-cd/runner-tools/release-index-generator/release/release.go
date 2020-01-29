package release

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"

	"gitlab.com/gitlab-org/ci-cd/runner-tools/release-index-generator/fs"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/release-index-generator/gpg"
)

const (
	indexFileName              = "index.html"
	checksumsFileName          = "release.sha256"
	checksumsSignatureFileName = "release.sha256.asc"
)

type Generator interface {
	Prepare() error
	GenerateIndexFile() error
}

type defaultGenerator struct {
	indexFile              string
	checksumsFile          string
	checksumsSignatureFile string

	info *Info

	directoryScanner fs.Scanner
	signer           gpg.Signer
}

func NewGenerator(workingDirectory string, info Info, signer gpg.Signer) Generator {
	info.directoryScanner = fs.NewDirectoryScanner(workingDirectory)

	return &defaultGenerator{
		indexFile:              filepath.Join(workingDirectory, indexFileName),
		checksumsFile:          filepath.Join(workingDirectory, checksumsFileName),
		checksumsSignatureFile: filepath.Join(workingDirectory, checksumsSignatureFileName),
		info:                   &info,
		directoryScanner:       info.directoryScanner,
		signer:                 signer,
	}
}

func (r *defaultGenerator) Prepare() error {
	steps := map[string]func() error{
		"scan working directory": r.scanWorkingDirectory,
		"prepare checksums file": r.prepareChecksumsFile,
		"sign checksums file":    r.signChecksumsFile,
	}

	for stepName, step := range steps {
		err := step()
		if err != nil {
			return fmt.Errorf("error while executing step %q: %w", stepName, err)
		}
	}

	return nil
}

func (r *defaultGenerator) scanWorkingDirectory() error {
	err := r.directoryScanner.Scan(func(_ string, fileInfo os.FileInfo) bool {
		if shouldSkipFile(fileInfo.Name()) {
			return false
		}

		return true
	})

	if err != nil {
		return fmt.Errorf("failed to scanWorkingDirectory the working directory")
	}

	return nil
}

func shouldSkipFile(fileName string) bool {
	return fileName == indexFileName ||
		fileName == checksumsFileName ||
		fileName == checksumsSignatureFileName
}

func (r *defaultGenerator) prepareChecksumsFile() error {
	f, err := os.Create(r.checksumsFile)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", r.checksumsFile, err)
	}
	defer f.Close()

	err = r.directoryScanner.Walk(func(entry fs.FileEntry) error {
		if shouldSkipFile(entry.FileName) {
			return nil
		}

		_, err := fmt.Fprintf(f, "%s\t%s\n", entry.Checksum, entry.RelativePath)
		if err != nil {
			return fmt.Errorf("failed to write to %q: %w", r.checksumsFile, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error while generating checksums file: %w", err)
	}

	err = r.directoryScanner.AddFile(r.checksumsFile)
	if err != nil {
		return fmt.Errorf("failed to add checksums file to the list: %w", err)
	}

	return nil
}

func (r *defaultGenerator) signChecksumsFile() error {
	if r.signer == nil {
		return nil
	}

	err := r.signer.SignFile(r.checksumsFile, r.checksumsSignatureFile)
	if err != nil {
		return fmt.Errorf("error while signing checksums file: %w", err)
	}

	err = r.directoryScanner.AddFile(r.checksumsSignatureFile)
	if err != nil {
		return fmt.Errorf("failed to add checksums signature file to the list: %w", err)
	}

	return nil
}

func (r *defaultGenerator) GenerateIndexFile() error {
	err := r.info.prepareFiles()
	if err != nil {
		return fmt.Errorf("failed to prepare files list: %w", err)
	}

	tpl, err := template.New("release").Parse(indexTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse the template: %w", err)
	}

	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, r.info)
	if err != nil {
		return fmt.Errorf("failed to execute the template: %w", err)
	}

	err = ioutil.WriteFile(r.indexFile, buf.Bytes(), 0600)
	if err != nil {
		return fmt.Errorf("failed to write the index file %q: %w", r.indexFile, err)
	}

	return nil
}

var indexTemplate = `
{{ $title := (print .Project " :: " .Name) }}

<html>
    <head>
        <meta charset="utf-8/">
        <title>{{ $title }}</title>
        <style type="text/css">
            body {font-family: monospace; font-size: 14px; margin: 40px; padding: 0;}
            h1 {border-bottom: 1px solid #aaa; padding: 10px;}
            p {font-size: 0.85em; margin: 5px 10px;}
            p span {display: inline-block; font-weight: bold; width: 100px;}
            p a {color: #000; font-weight: bold; text-decoration: none;}
            p a:hover {text-decoration: underline;}
            ul {background: #eee; border: 1px solid #aaa; border-radius: 3px; box-shadow: 0 0 5px #aaa inset; list-style-type: none; margin: 10px 0; padding: 10px;}
            li {margin: 5px 0; padding: 5px; font-size: 12px;}
            li:hover {background: #dedede;}
            .file_name {display: inline-block; font-weight: bold; width: calc(100% - 610px);}
            .file_name a {color: #000; display: inline-block; text-decoration: none; width: calc(100% - 10px);}
            .file_checksum {display: inline-block; text-align: right; width: 500px;}
            .file_size {display: inline-block; text-align: right; width: 90px;}
        </style>
    </head>
    <body>
        <h1>{{ $title }}</h1>

        <p><span>Sources:</span> <a href="{{ .SourceURL }}" target="_blank">{{ .SourceURL }}</a></p>
        <p><span>Revision:</span> {{ .Revision }}</p>
        <p><span>Ref:</span> {{ .Ref }}</p>
        <p><span>Created at:</span> {{ .CreatedAt }}</p>

        <ul>
        {{ range $_, $file := .Files }}
            <li>
                <span class="file_name"><a href="./{{ $file.RelativePath }}">{{ $file.RelativePath }}</a></span>
                <span class="file_checksum">{{ $file.Checksum }}</span>
                <span class="file_size">{{ printf "%2.2f" $file.SizeMb }} MiB</span>
            </li>
        {{ end }}
        </ul>
    </body>
</html>
`
