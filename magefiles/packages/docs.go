package packages

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docutils"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	startSupportedOSDocs = "<!-- supported_os_versions_list_start -->"
	endSupportedOSDocs   = "<!-- supported_os_versions_list_end -->"
	docsFilePath         = "docs/install/linux-repository.md"
)

// This type is to reuse existing code that would otherwise cause a circular dependency.
type distListFunc func(string, string) ([]string, error)

func GenerateSupportedOSDocs(f distListFunc) error {
	debDists, rpmDists, err := getDistributionLists(f)
	if err != nil {
		return err
	}

	rendered := render(debDists, rpmDists)

	origContent, err := os.ReadFile(docsFilePath)
	if err != nil {
		return err
	}

	newContent, err := replace(
		startSupportedOSDocs,
		endSupportedOSDocs,
		string(origContent),
		rendered)
	if err != nil {
		return err
	}

	if err := os.WriteFile(docsFilePath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("error while writing new content for %q file: %w", origContent, err)
	}

	return nil
}

func getDistributionLists(f distListFunc) ([]string, []string, error) {
	debOSs, derr := f("deb", "stable")
	rpmOSs, rerr := f("rpm", "stable")
	return debOSs, rpmOSs, errors.Join(derr, rerr)
}

func render(debDists, rpmDists []string) string {
	buf := strings.Builder{}
	buf.WriteString(startSupportedOSDocs)

	buf.WriteString("\n### Deb-based Distributions\n\n")
	renderTable(debDists, &buf)

	buf.WriteString("\n### Rpm-based Distributions\n\n")
	renderTable(rpmDists, &buf)

	buf.WriteString("\n")
	buf.WriteString(endSupportedOSDocs)
	buf.WriteString("\n")

	return buf.String()
}

var properDistNames = map[string]string{
	"ubuntu":    "Ubuntu",
	"debian":    "Debian",
	"linuxmint": "LinuxMint",
	"raspbian":  "Raspbian",
	"el":        "Red Hat Enterprise Linux",
	"fedora":    "Fedora",
	"ol":        "Oracle Linux",
	"opensuse":  "openSUSE",
	"sles":      "SUSE Linux Enterprise Server",
	"amazon":    "Amazon Linux",
}

//nolint:errcheck
func renderTable(dists []string, dest io.StringWriter) {
	versByOS := map[string][]string{}
	for _, f := range dists {
		toks := strings.Split(f, "/")
		os := toks[0]
		ver := cases.Title(language.English, cases.Compact).String(toks[1])

		versByOS[os] = append(versByOS[os], ver)
	}

	dest.WriteString("| Distribution | Supported Versions |\n")
	dest.WriteString("|--------------|--------------------|\n")

	for _, dist := range slices.Sorted(maps.Keys(versByOS)) {
		vers := versByOS[dist]
		dist = properDistNames[dist]
		dest.WriteString("| ")
		dest.WriteString(dist)
		dest.WriteString(" | ")
		dest.WriteString(strings.Join(vers, ", "))
		dest.WriteString(" |\n")
	}
}

func replace(placeholderStart, placeholderEnd, fileContent, content string) (string, error) {
	replacer := docutils.NewBlockLineReplacer(placeholderStart, placeholderEnd, fileContent, content)

	newContent, err := replacer.Replace()
	if err != nil {
		return "", fmt.Errorf("error while replacing the content: %w", err)
	}

	return newContent, nil
}
