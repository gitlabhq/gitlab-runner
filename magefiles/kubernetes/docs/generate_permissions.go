package docs

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"text/template"

	"github.com/samber/lo"
)

const (
	startTablePlaceholder    = "<!-- k8s_api_permissions_list_start -->"
	endTablePlaceholder      = "<!-- k8s_api_permissions_list_end -->"
	startRoleYamlPlaceholder = "<!-- k8s_api_permissions_role_yaml_start -->"
	endRoleYamlPlaceholder   = "<!-- k8s_api_permissions_role_yaml_end -->"
	docsFilePath             = "docs/executors/kubernetes/index.md"
)

var tableTemplate = fmt.Sprintf(` %s

| Resource | Verb (Optional Feature/Config Flags) |
|----------|-------------------------------|
{{ range $_, $permissions := . -}}
| {{ $permissions.Resource }} | {{ $permissions.Verbs | joinVerbs }} |
{{ end }}
%s
`, startTablePlaceholder, endTablePlaceholder)

type permissionsRender struct {
	Resource string
	Verbs    []verb
}

func ParsePermissions() (PermissionsGroup, error) {
	return parsePermissions("executors/kubernetes", filterTestFiles)
}

func GeneratePermissionsDocs(roleName, roleNamespace string, permissions PermissionsGroup) error {
	docsFile, err := os.ReadFile(docsFilePath)
	if err != nil {
		return err
	}

	table, err := renderTable(mergePermissions(permissions))
	if err != nil {
		return err
	}

	newDocsFile, err := replace(startTablePlaceholder, endTablePlaceholder, string(docsFile), table)
	if err != nil {
		return err
	}

	if err := os.WriteFile(docsFilePath, []byte(newDocsFile), 0o644); err != nil {
		return fmt.Errorf("error while writing new content for %q file: %w", docsFile, err)
	}

	roleYaml, err := GeneratePermissionsDocsRoleYaml(roleName, roleNamespace, nil, permissions)
	if err != nil {
		return err
	}

	roleYamlContent := fmt.Sprintf("%s\n```yaml\n%s\n```\n%s\n", startRoleYamlPlaceholder, strings.TrimSpace(roleYaml), endRoleYamlPlaceholder)

	newDocsFile, err = replace(startRoleYamlPlaceholder, endRoleYamlPlaceholder, newDocsFile, roleYamlContent)
	if err != nil {
		return err
	}

	if err := os.WriteFile(docsFilePath, []byte(newDocsFile), 0o644); err != nil {
		return fmt.Errorf("error while writing new content for %q file: %w", docsFile, err)
	}

	return nil
}

func GeneratePermissionsDocsRoleYaml(roleName, roleNamespace string, roleLabels map[string]string, permissions PermissionsGroup) (string, error) {
	roleTemplateBytes, err := os.ReadFile("magefiles/kubernetes/docs/role.yaml.tpl")
	if err != nil {
		return "", fmt.Errorf("error reading role template: %w", err)
	}

	tpl := template.New("roleTemplate")
	tpl.Funcs(template.FuncMap{
		"joinConfigFlags": func(input []configFlag) string {
			flags := lo.Map(input, func(item configFlag, _ int) string {
				return item.String()
			})
			sort.Strings(flags)

			return strings.Join(flags, ", ")
		},
	})

	tpl, err = tpl.Parse(string(roleTemplateBytes))
	if err != nil {
		return "", fmt.Errorf("error parsing role template: %w", err)
	}

	buffer := new(bytes.Buffer)
	err = tpl.Execute(buffer, map[string]any{
		"Name":      roleName,
		"Namespace": roleNamespace,
		"Rules":     permissions,
		"Labels":    roleLabels,
	})
	if err != nil {
		return "", fmt.Errorf("error executing role template: %w", err)
	}

	return buffer.String(), nil
}

func mergePermissions(permissions PermissionsGroup) []permissionsRender {
	render := lo.Map(lo.Keys(permissions), func(key string, _ int) permissionsRender {
		return permissionsRender{
			Resource: key,
			Verbs:    permissions[key],
		}
	})

	slices.SortFunc(render, func(i, j permissionsRender) int {
		return strings.Compare(i.Resource, j.Resource)
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

func replace(placeholderStart, placeholderEnd, fileContent, content string) (string, error) {
	replacer := NewBlockLineReplacer(placeholderStart, placeholderEnd, fileContent, content)

	newContent, err := replacer.Replace()
	if err != nil {
		return "", fmt.Errorf("error while replacing the content: %w", err)
	}

	return newContent, nil
}
