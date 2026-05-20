package main

import (
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

// Known architectures for stripping arch info from tags.
var knownArchs = []string{"arm64", "arm", "ppc64le", "riscv64", "s390x", "x86_64"}

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
		if strings.Contains(componentName, rule.windowsFlavor) {
			return rule.tagTemplate
		}
	}
	return ""
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

// stripTags runs stripTag on the given tags and returns the collected result.
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
func collectIndexes(m *Manifest) IndexMap {
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
	for componentName, tags := range m.Default {
		strippedTags := stripTags(tags)

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

	return indexes
}

// generateIndexes creates configuration for image indexes.
// To reduce configuration burden when adding/updating component images to push,
// simple rules are followed to combine pushed component images into a set of
// reasonable image indexes.
func generateIndexes(m *Manifest) []ImageIndex {
	indexMap := collectIndexes(m)
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
