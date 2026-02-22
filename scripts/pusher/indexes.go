package main

import (
	"slices"
	"sort"
	"strings"
)

// ImageIndex represents a group of archives that should be included in an index
type ImageIndex struct {
	Tags       []string `json:"tags"`
	Components []string `json:"components"`
}

// Map from tagsKey(tagTemplates) to the ImageIndex containing those tags.
// Used to collate the separate components in the config file to the appropriate
// ImageIndex composite values.
type IndexMap map[string]*ImageIndex

// Known architectures for stripping arch info from tags
var knownArchs = []string{"arm64", "arm", "ppc64le", "riscv64", "s390x", "x86_64"}

// Choose the windows archives that should be included in the default "%" index
func isWindowsDefaultArchive(componentName string) bool {
	return strings.Contains(componentName, "nanoserver")
}

// Determine whether a component belongs in the super index, e.g. gitlab-runner-helper:v18.10.0.
func checkIfShouldBeInDefault(componentName string, strippedTags []string) bool {
	for _, stripped := range strippedTags {
		if stripped == "%" {
			return true
		}
	}

	return isWindowsDefaultArchive(componentName)
}

// stripTag removes the architecture and windows os.version info from tag templates
// Makes the following assumptions:
//  1. No tag ends with an architecture identifier.
//  2. Windows tags all mention either nanoserver or servercore
//  3. Any text _after_ nanoserver or servercore is a version identifier
//     for the windows tag, and should be excluded from the index tag
//
// Examples:
//
//	"x86_64-%" -> "%"
//	"alpine3.21-x86_64-%" -> "alpine3.21-%"
//	"x86_64-%-pwsh" -> "%-pwsh"
//	"x86_64-%-servercore1809" -> "%-servercore"
func stripTag(tag string) string {
	for _, arch := range knownArchs {
		archSegment := arch + "-"
		if strings.Contains(tag, archSegment) {
			stripped := strings.Replace(tag, archSegment, "", 1)

			for _, winVariant := range []string{"servercore", "nanoserver"} {
				if idx := strings.Index(stripped, winVariant); idx != -1 {
					// If we found the variant, trim the stripped content
					// to everything up until the end of the variant name
					stripped = stripped[:idx+len(winVariant)]
				}
			}

			return stripped
		}
	}

	return tag
}

// Run stripTag on the inputTags and return the collected result
func stripTags(tags []string) []string {
	var result []string

	for _, tag := range tags {
		result = append(result, stripTag(tag))
	}

	return result
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
func (indexes IndexMap) Add(tags []string, archiveName string) {
	sort.Strings(tags)
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
func collectIndexes(m *Manifest) IndexMap {
	indexes := make(IndexMap)

	// Note: We only generate indexes based on the "Default" component config.
	//
	// The manifest does support configuring some components to be pushed based on specific
	// tag fragments given on the command line, via the m.match(tagFragment) function.
	// This feature doesn't appear to be used in the current config file, and is entirely
	// ignored here.
	for componentName, tags := range m.Default {
		strippedTags := stripTags(tags)

		// Filter out "%" from the regular group tags
		var nonDefaultTags []string
		for _, tag := range strippedTags {
			// We ignore the default tag template of "%" during normal processing to separate
			// the super index from the other tags.
			if tag != "%" {
				nonDefaultTags = append(nonDefaultTags, tag)
			}
		}

		// Add to the non-default group if there are any non-default tags
		if len(nonDefaultTags) > 0 {
			indexes.Add(nonDefaultTags, componentName)
		}

		// Add component to the super index if appropriate
		if checkIfShouldBeInDefault(componentName, strippedTags) {
			indexes.Add([]string{"%"}, componentName)
		}
	}

	return indexes
}

// GenerateIndexes automatically generates index manifests from the default map
func GenerateIndexes(m *Manifest) []ImageIndex {
	indexMap := collectIndexes(m)
	var indexes []ImageIndex
	for _, index := range indexMap {
		// We sort the components to ensure deterministic ordering in the resulting image index
		sort.Strings(index.Components)
		indexes = append(indexes, *index)
	}

	// We sort the resulting ImageIndex values to make validation easier.
	slices.SortFunc(indexes, func(a, b ImageIndex) int {
		return strings.Compare(a.Tags[0], b.Tags[0])
	})
	return indexes
}
