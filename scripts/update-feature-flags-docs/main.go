package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"text/template"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

const (
	docsFile = "./docs/configuration/feature-flags.md"

	startPlaceholder = "<!-- feature_flags_list_start -->"
	endPlaceholder   = "<!-- feature_flags_list_end -->"
)

var ffTableTemplate = `{{ placeholder "start" }}

| Feature flag | Default value | Deprecated | To be removed with | Description |
|--------------|---------------|------------|--------------------|-------------|
{{ range $_, $flag := . -}}
| {{ $flag.Name | raw }} | {{ $flag.DefaultValue | raw }} | {{ $flag.Deprecated | tick }} | {{ $flag.ToBeRemovedWith }} | {{ $flag.Description }} |
{{ end }}
{{ placeholder "end" }}
`

func main() {
	fileContent := getFileContent()
	tableContent := prepareTable()

	newFileContent := replace(fileContent, tableContent)

	saveFileContent(newFileContent)
}

func getFileContent() string {
	data, err := ioutil.ReadFile(docsFile)
	if err != nil {
		panic(fmt.Sprintf("Error while reading file %q: %v", docsFile, err))
	}

	return string(data)
}

func prepareTable() string {
	tpl := template.New("ffTable")
	tpl.Funcs(template.FuncMap{
		"placeholder": func(placeholderType string) string {
			switch placeholderType {
			case "start":
				return startPlaceholder
			case "end":
				return endPlaceholder
			default:
				panic(fmt.Sprintf("Undefined placeholder type %q", placeholderType))
			}
		},
		"raw": func(input string) string {
			return fmt.Sprintf("`%s`", input)
		},
		"tick": func(input bool) string {
			if input {
				return "✓"
			}

			return "✗"
		},
	})

	tpl, err := tpl.Parse(ffTableTemplate)
	if err != nil {
		panic(fmt.Sprintf("Error while parsing the template: %v", err))
	}

	buffer := new(bytes.Buffer)

	err = tpl.Execute(buffer, featureflags.GetAll())
	if err != nil {
		panic(fmt.Sprintf("Error while executing the template: %v", err))
	}

	return buffer.String()
}

func replace(fileContent string, tableContent string) string {
	replacer := newBlockLineReplacer(startPlaceholder, endPlaceholder, fileContent, tableContent)

	newContent, err := replacer.Replace()
	if err != nil {
		panic(fmt.Sprintf("Error while replacing the content: %v", err))
	}

	return newContent
}

func saveFileContent(newFileContent string) {
	err := ioutil.WriteFile(docsFile, []byte(newFileContent), 0644)
	if err != nil {
		panic(fmt.Sprintf("Error while writing new content for %q file: %v", docsFile, err))
	}
}

type blockLineReplacer struct {
	startLine      string
	endLine        string
	replaceContent string

	input  *bytes.Buffer
	output *bytes.Buffer

	startFound bool
	endFound   bool
}

func (r *blockLineReplacer) Replace() (string, error) {
	for {
		line, err := r.input.ReadString('\n')
		if err == io.EOF {
			break
		}

		if err != nil {
			return "", fmt.Errorf("error while reading issue description: %w", err)
		}

		r.handleLine(line)
	}

	return r.output.String(), nil
}

func (r *blockLineReplacer) handleLine(line string) {
	r.handleStart(line)
	r.handleRewrite(line)
	r.handleEnd(line)
}

func (r *blockLineReplacer) handleStart(line string) {
	if r.startFound || !strings.Contains(line, r.startLine) {
		return
	}

	r.startFound = true
}

func (r *blockLineReplacer) handleRewrite(line string) {
	if r.startFound && !r.endFound {
		return
	}

	r.output.WriteString(line)
}

func (r *blockLineReplacer) handleEnd(line string) {
	if !strings.Contains(line, r.endLine) {
		return
	}

	r.endFound = true
	r.output.WriteString(r.replaceContent)
}

func newBlockLineReplacer(startLine string, endLine string, input string, replaceContent string) *blockLineReplacer {
	return &blockLineReplacer{
		startLine:      startLine,
		endLine:        endLine,
		input:          bytes.NewBufferString(input),
		output:         new(bytes.Buffer),
		replaceContent: replaceContent,
		startFound:     false,
		endFound:       false,
	}
}
