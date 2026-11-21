//go:build mage

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"text/template"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docutils"
)

// UpdateFeatureFlagsDocs regenerates the feature flags table in the documentation.
func UpdateFeatureFlagsDocs() error {
	const docsFilePath = "docs/configuration/feature-flags.md"
	const tableTemplate = `
| Feature flag | Default value | Deprecated | To be removed with | Description |
|--------------|---------------|------------|--------------------|-------------|
{{ range $_, $flag := . -}}
| {{ $flag.Name | raw }} | {{ $flag.DefaultValue | bool }} | {{ $flag.Deprecated | tick }} | {{ $flag.ToBeRemovedWith }} | {{ $flag.Description }} |
{{ end }}
`

	tpl, err := template.New("ffTable").Funcs(template.FuncMap{
		"raw": func(input string) string {
			return fmt.Sprintf("`%s`", input)
		},
		"bool": func(input bool) string {
			return fmt.Sprintf("`%t`", input)
		},
		"tick": func(input bool) string {
			if input {
				return "{{< icon name=\"check-circle\" >}} Yes"
			}

			return "{{< icon name=\"dotted-circle\" >}} No"
		},
	}).Parse(tableTemplate)
	if err != nil {
		return fmt.Errorf("parsing feature flags table template: %w", err)
	}

	fileContent, err := os.ReadFile(docsFilePath)
	if err != nil {
		return fmt.Errorf("reading feature flags documentation: %w", err)
	}

	replacer := docutils.NewSectionReplacer("feature_flags_list", bytes.NewReader(fileContent))
	err = replacer.Replace(func(_ io.Reader) (string, error) {
		// DeleteFunc mutates its input; preserve the shared feature flag definitions.
		ffs := slices.DeleteFunc(slices.Clone(featureflags.GetAll()), func(ff featureflags.FeatureFlag) bool {
			return ff.Name == "FF_TEST_FEATURE"
		})

		var buffer bytes.Buffer
		if err := tpl.Execute(&buffer, ffs); err != nil {
			return "", fmt.Errorf("rendering feature flags table: %w", err)
		}

		return buffer.String(), nil
	})
	if err != nil {
		return fmt.Errorf("replacing feature flags table: %w", err)
	}

	if err := os.WriteFile(docsFilePath, []byte(replacer.Output()), 0o644); err != nil {
		return fmt.Errorf("writing feature flags documentation: %w", err)
	}

	return nil
}
