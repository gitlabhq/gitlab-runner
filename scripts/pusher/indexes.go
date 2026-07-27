package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// ImageIndex represents a group of archives that should be included in an index.
type ImageIndex struct {
	Tags       []string `json:"tags"`
	Components []string `json:"components"`
}

// Map from tagsKey(tagTemplates) to the ImageIndex containing those tags.
// Used to collate the separate components in the config file to the appropriate
// ImageIndex composite values.
type IndexMap map[string]*ImageIndex

// Known tag components for stripping component tags down to index tags.
// If a new architecture, flavor, windows version, or suffix is added to the build via
// dockerfiles/runner-helper/docker-bake.hcl and scripts/pusher/helper-images.json, it'll
// have to be added here in order to be included in a helper image index. Tags referenced
// in scripts/pusher/helper-images.json that violate these expectations will cause a test
// failure. Additionally, knownWinVersions should track the Windows version constants in
// helpers/container/windows/version.go and helpers/container/helperimage/windows_info.go.
// See: https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39597.
var (
	knownArchs         = []string{"arm", "arm64", "ppc64le", "riscv64", "s390x", "x86_64"}
	knownLinuxFlavors  = []string{"alpine-edge", "alpine-latest", "alpine3.21", "concrete", "ubi-fips", "ubuntu"}
	knownLinuxSuffixes = []string{"pwsh"}
	knownWinFlavors    = []string{"servercore", "nanoserver"}
	knownWinVersions   = []string{"1809", "21H2", "24H2"}
)

// crossOsRule identifies a tag template that should be handled as a cross-OS tag,
// and a component name fragment (e.g. "nanoserver") that identifies components that
// should be included in that cross-OS tag.
type CrossOsRule struct {
	tagTemplate   string
	windowsFlavor string
}

// crossOsRules encapsulates the matching rules for cross-OS image indexes.
// Rules are stored in registration order to ensure deterministic matching.
type CrossOsRules []CrossOsRule

// addRule adds a rule, stating that:
//  1. The tag template should be handled separately from simple tags which have no rules.
//  2. Components containing the windows flavor should be included in the cross-OS index
//     associated with that tag template.
func (r *CrossOsRules) addRule(tagTemplate, windowsFlavor string) {
	*r = append(*r, CrossOsRule{tagTemplate: tagTemplate, windowsFlavor: windowsFlavor})
}

// hasRule returns true if the given tag template matches any current rule.
func (r *CrossOsRules) hasRule(tagTemplate string) bool {
	for _, rule := range *r {
		if rule.tagTemplate == tagTemplate {
			return true
		}
	}
	return false
}

// tagFor returns the cross-OS tag template that should be used for the given component
// with the given tags, or "" if no rules matches.
func (r *CrossOsRules) tagFor(componentName string, compTagTemplates []string) string {
	for _, rule := range *r {
		if slices.Contains(compTagTemplates, rule.tagTemplate) {
			return rule.tagTemplate
		}
		if len(compTagTemplates) > 0 && strings.Contains(componentName, rule.windowsFlavor) {
			return rule.tagTemplate
		}
	}
	return ""
}

// stripTag removes the architecture and windows os.version info from tag templates
// Makes the following assumptions:
// 1. Linux tags follow the format: [flavor-]{arch}-%[-{suffix}]
// 2. Windows tags follow the format: {arch}-%-{winFlavor}{winVersion}
//
// Any tags which do not follow one of these formats will generate an error.
//
// For tags that do follow the pattern, we strip the {arch} and {winVersion} components to make
// the final index tag.
//
// Examples:
//
//	"x86_64-%" -> "%"
//	"alpine3.21-x86_64-%" -> "alpine3.21-%"
//	"x86_64-%-pwsh" -> "%-pwsh"
//	"x86_64-%-servercore1809" -> "%-servercore"
func stripTag(tag string) (string, error) {
	for _, arch := range knownArchs {
		archSegment := arch + "-"
		if before, after, found := strings.Cut(tag, archSegment); found {
			if matchingTag := matchesLinuxTagFormat(before, after); matchingTag != "" {
				return matchingTag, nil
			} else if matchingTag := matchesWindowsTagFormat(before, after); matchingTag != "" {
				return matchingTag, nil
			}
		}
	}

	return "", fmt.Errorf("cannot strip tag %s", tag)
}

func matchesLinuxTagFormat(prefix, suffix string) string {
	if (prefix == "" || slices.Contains(knownLinuxFlavors, strings.TrimSuffix(prefix, "-"))) &&
		(suffix == "%" || slices.Contains(knownLinuxSuffixes, strings.TrimPrefix(suffix, "%-"))) {
		return prefix + suffix
	}
	return ""
}

func matchesWindowsTagFormat(prefix, suffix string) string {
	if prefix != "" {
		// all known windows tags currently have no prefix
		return ""
	}
	for _, winFlavor := range knownWinFlavors {
		winFlavorTag := "%-" + winFlavor
		for _, winVersion := range knownWinVersions {
			if suffix == winFlavorTag+winVersion {
				return winFlavorTag
			}
		}
	}
	return ""
}

// stripTags runs stripTag on the given tags and returns the collected result.
func stripTags(tags []string) ([]string, []string) {
	var result []string
	var rejectedTags []string

	for _, tag := range tags {
		strippedTag, err := stripTag(tag)
		if err != nil {
			rejectedTags = append(rejectedTags, tag)
		} else {
			result = append(result, strippedTag)
		}
	}

	return result, rejectedTags
}

// tagsKey creates a unique grouping key from an ordered tag set.
func tagsKey(tags []string) string {
	return strings.Join(tags, "|")
}

// Add archive/tag data to the index map.
//
// Operates by either creating a new ImageIndex containing the input archive as
// the only component, or appending that component to the existing ImageIndex.
// Sorts the given tags slice as a side-effect of the operation.
func (indexes IndexMap) add(tags []string, archiveName string) {
	slices.Sort(tags)
	indexKey := tagsKey(tags)

	if index, exists := indexes[indexKey]; exists {
		index.Components = append(index.Components, archiveName)
	} else {
		indexes[indexKey] = &ImageIndex{
			Tags:       tags,
			Components: []string{archiveName},
		}
	}
}

// Group the component/tag data in the config file into a map of appropriate
// indexes, with map key based on the set of stripped tags associated with
// the component.
func collectIndexes(m *Manifest) (IndexMap, []string) {
	indexes := make(IndexMap)
	crossOs := CrossOsRules{}

	crossOs.addRule("%", "servercore")
	crossOs.addRule("%-pwsh", "nanoserver")

	// Note: We only generate indexes based on the "Default" component config.
	//
	// The manifest does support configuring some components to be pushed based on specific
	// tag fragments given on the command line, via the m.match(tagFragment) function.
	// This feature doesn't appear to be used in the current config file, and is entirely
	// ignored here.
	var tagsRejected []string
	for componentName, tags := range m.Default {
		strippedTags, rejects := stripTags(tags)
		tagsRejected = append(tagsRejected, rejects...)

		// Filter out cross-OS tags from the simple tags, as cross-OS tags are handled
		// separately, below.
		var simpleTags []string
		for _, tag := range strippedTags {
			if !crossOs.hasRule(tag) {
				simpleTags = append(simpleTags, tag)
			}
		}

		// Add the component to an index with all its simple tags, if it has any.
		if len(simpleTags) > 0 {
			indexes.add(simpleTags, componentName)
		}

		// Now add the component to the appropriate cross-OS index, if the rules
		// indicate we should.
		if crossOsTag := crossOs.tagFor(componentName, strippedTags); crossOsTag != "" {
			indexes.add([]string{crossOsTag}, componentName)
		}
	}
	return indexes, tagsRejected
}

// generateIndexes creates configuration for image indexes.
//
// To reduce configuration burden when adding/updating component images to push,
// simple rules are followed to combine pushed component images into a set of
// reasonable image indexes.
//
// To automatically combine those component images into an appropriate set of image
// indexes, we make a few assumptions:
//
//  1. Every supported linux tag is of the format [flavor-]{arch}-%[-{suffix}], e.g.
//     alpine-latest-x86_64-%, or ubuntu-arm64-%-pwsh.
//  2. Every supported windows tag is of the format {arch}-%-{winFlavor}{winVersion},
//     e.g. x86_64-%-servercore21H2 or x86_64-%-nanoserver1809.
//
// With those assumptions in place, our baseline rule is fairly simple: Tags that
// differ only on architecture or Windows version should be placed in an image index
// together, with a tag name derived by simply omitting architecture and Windows
// version specifiers. For example ubuntu-%-pwsh or %-servercore.
//
// If the target image index tag is simply % or %-pwsh, it is handled as a cross-OS
// index, including both the linux image that was to be pushed as well as servercore
// images for % and nanoserver images for %-pwsh.
//
// Any component tag found in the manifest which does not match the expected conventions
// is written as a warning to w.
func generateIndexes(m *Manifest, w io.Writer) []ImageIndex {
	indexMap, rejectedTags := collectIndexes(m)
	if len(rejectedTags) > 0 {
		fmt.Fprintf(w, "warning: ignoring unrecognized tag(s): %v\n", rejectedTags)
	}
	var indexes []ImageIndex
	for _, index := range indexMap {
		// We sort the components to ensure deterministic ordering in the resulting image index
		slices.Sort(index.Components)
		indexes = append(indexes, *index)
	}

	// We sort the resulting ImageIndex values to make validation easier.
	slices.SortFunc(indexes, func(a, b ImageIndex) int {
		return strings.Compare(a.Tags[0], b.Tags[0])
	})
	return indexes
}
