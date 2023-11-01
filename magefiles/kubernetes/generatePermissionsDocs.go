package kubernetes

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/samber/lo"
	"golang.org/x/exp/slices"
)

const (
	startPlaceholder = "<!-- k8s_api_permissions_list_start -->"
	endPlaceholder   = "<!-- k8s_api_permissions_list_end -->"
	docsFilePath     = "docs/executors/kubernetes.md"
)

var tableTemplate = fmt.Sprintf(` %s

| Resource | Verb (Optional Feature Flags) |
|----------|-------------------------------|
{{ range $_, $permissions := . -}}
| {{ $permissions.Resource }} | {{ $permissions.Verbs | joinVerbs }} |
{{ end }}
%s
`, startPlaceholder, endPlaceholder)

type permissionsRender struct {
	Resource string
	Verbs    []verb
}

func GeneratePermissionsDocs() error {
	permissions, err := parsePermissions()
	if err != nil {
		return err
	}

	docsFile, err := os.ReadFile(docsFilePath)
	if err != nil {
		return err
	}

	table, err := renderTable(mergePermissions(permissions))
	if err != nil {
		return err
	}

	newDocsFile, err := replace(string(docsFile), table)
	if err != nil {
		return err
	}

	if err := os.WriteFile(docsFilePath, []byte(newDocsFile), 0o644); err != nil {
		return fmt.Errorf("error while writing new content for %q file: %w", docsFile, err)
	}

	return nil
}

func mergePermissions(permissions permissionsGroup) []permissionsRender {
	render := lo.Map(lo.Keys(permissions), func(key string, _ int) permissionsRender {
		return permissionsRender{
			Resource: key,
			Verbs:    permissions[key],
		}
	})

	slices.SortFunc(render, func(i, j permissionsRender) bool {
		return strings.Compare(i.Resource, j.Resource) < 0
	})

	return render
}

func renderTable(permissions []permissionsRender) (string, error) {
	tpl := template.New("permissionsTable")
	tpl.Funcs(template.FuncMap{
		"joinVerbs": func(input []verb) string {
			verbs := lo.Map(input, func(item verb, _ int) string {
				return item.String()
			})
			sort.Strings(verbs)

			return strings.Join(verbs, ", ")
		},
	})

	tpl, err := tpl.Parse(tableTemplate)
	if err != nil {
		return "", err
	}

	buffer := new(bytes.Buffer)

	err = tpl.Execute(buffer, permissions)
	if err != nil {
		return "", fmt.Errorf("error while executing the template: %w", err)
	}

	return buffer.String(), nil
}

func replace(fileContent, tableContent string) (string, error) {
	replacer := newBlockLineReplacer(startPlaceholder, endPlaceholder, fileContent, tableContent)

	newContent, err := replacer.Replace()
	if err != nil {
		return "", fmt.Errorf("error while replacing the content: %w", err)
	}

	return newContent, nil
}
