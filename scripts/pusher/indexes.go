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

type IndexMap map[string]*ImageIndex

// Known architectures for stripping arch info from tags
var knownArchs = []string{"arm64", "arm", "ppc64le", "riscv64", "s390x", "x86_64"}

// stripTag removes the architecture and windows os.version info from tag
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

// tagsKey creates a unique grouping key from a sorted tag set.
// We sort the tags so that components which list their tags in inconsistent
// order can still be grouped.
func tagsKey(tags []string) string {
	sort.Strings(tags)
	return strings.Join(tags, "|")
}

// Choose the windows archives that should be included in the default "%" index
func isWindowsDefaultArchive(archiveName string) bool {
	return strings.HasPrefix(archiveName, "windows-nanoserver")
}

// Add archive/tag data to the map, either creating a new ImageIndex containing
// the input archive as the only component, or appending that component to the
// existing ImageIndex.
func (indexes IndexMap) Add(tags []string, archiveName string) {
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

func checkIfShouldBeInDefault(archiveName string, strippedTags []string) bool {
	for _, stripped := range strippedTags {
		if stripped == "%" {
			return true
		}
	}

	return isWindowsDefaultArchive(archiveName)
}

// Group the component/tag data in the config file into a map of appropriate
// indexes, with map key based on the set of stripped tags associated with
// the component.
func collectIndexes(defaultMap map[string][]string) IndexMap {
	indexes := make(IndexMap)

	for archiveName, tags := range defaultMap {
		strippedTags := stripTags(tags)

		// Check if this should be in the default index
		shouldBeInDefault := checkIfShouldBeInDefault(archiveName, strippedTags)

		// Filter out "%" from the regular group tags
		var nonDefaultTags []string
		for _, tag := range strippedTags {
			if tag != "%" {
				nonDefaultTags = append(nonDefaultTags, tag)
			}
		}

		// Add to the non-default group if there are any non-default tags
		if len(nonDefaultTags) > 0 {
			indexes.Add(nonDefaultTags, archiveName)
		}

		// Add to default group if needed
		if shouldBeInDefault {
			indexes.Add([]string{"%"}, archiveName)
		}
	}

	return indexes
}

// GenerateIndexes automatically generates index manifests from the default map
func GenerateIndexes(m *Manifest) []ImageIndex {
	indexMap := collectIndexes(m.Default)

	var indexes []ImageIndex
	for _, index := range indexMap {
		sort.Strings(index.Components)
		indexes = append(indexes, *index)
	}
	slices.SortFunc(indexes, func(a, b ImageIndex) int {
		return strings.Compare(a.Tags[0], b.Tags[0])
	})
	return indexes
}
