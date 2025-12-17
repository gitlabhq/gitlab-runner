//go:build !integration

package helpers

import (
	"path/filepath"
	"testing"
)

func TestToBackslash(t *testing.T) {
	result := ToBackslash("smb://user/me/directory")
	expected := "smb:\\\\user\\me\\directory"

	if result != expected {
		t.Error("Expected", expected, ", got ", result)
	}
}

func TestToSlash(t *testing.T) {
	result := ToSlash("smb:\\\\user\\me\\directory")
	expected := "smb://user/me/directory"

	if result != expected {
		t.Error("Expected", expected, ", got ", result)
	}
}

func TestIsImmediateChild(t *testing.T) {
	// Use filepath.Join to create OS-appropriate paths
	// This makes tests work correctly on both Unix and Windows

	tests := []struct {
		name     string
		parent   string
		child    string
		expected bool
	}{
		{
			name:     "immediate child",
			parent:   filepath.Join("home", "user", "VirtualBox VMs"),
			child:    filepath.Join("home", "user", "VirtualBox VMs", "Ubuntu"),
			expected: true,
		},
		{
			name:     "nested child two levels deep",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("home", "user", "documents", "projects"),
			expected: false,
		},
		{
			name:     "same directory",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("home", "user"),
			expected: false,
		},
		{
			name:     "parent directory",
			parent:   filepath.Join("home", "user", "documents"),
			child:    filepath.Join("home", "user"),
			expected: false,
		},
		{
			name:     "sibling directory",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("home", "otheruser"),
			expected: false,
		},
		{
			name:     "completely unrelated path",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("etc", "config"),
			expected: false,
		},
		{
			name:     "similar prefix but not child",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("home", "userdata"),
			expected: false,
		},
		{
			name:     "path traversal with double dots",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("home", "user", "..", "otheruser"),
			expected: false,
		},
		{
			name:     "complex path traversal",
			parent:   filepath.Join("home", "user", "safe"),
			child:    filepath.Join("home", "user", "safe", "..", "..", "etc"),
			expected: false,
		},
		{
			name:     "traversal that returns to same parent",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("home", "user", "documents", "..", "downloads"),
			expected: true, // This actually resolves to an immediate child
		},
		{
			name:     "traversal that goes up and back",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("home", "user", "..", "user", "documents"),
			expected: true, // Resolves to immediate child after cleaning
		},
		{
			name:     "single dot in path",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("home", "user", ".", "documents"),
			expected: true, // Single dot is removed during cleaning
		},
		{
			name:     "multiple single dots",
			parent:   filepath.Join("home", ".", "user"),
			child:    filepath.Join("home", ".", "user", ".", "documents"),
			expected: true, // All single dots removed during cleaning
		},
		{
			name:     "relative immediate child",
			parent:   filepath.Join("user", "data"),
			child:    filepath.Join("user", "data", "cache"),
			expected: true,
		},
		{
			name:     "relative nested child",
			parent:   filepath.Join("user", "data"),
			child:    filepath.Join("user", "data", "cache", "temp"),
			expected: false,
		},
		{
			name:     "relative parent path",
			parent:   filepath.Join("user", "data", "cache"),
			child:    filepath.Join("user", "data"),
			expected: false,
		},
		{
			name:     "single component paths",
			parent:   "home",
			child:    filepath.Join("home", "user"),
			expected: true,
		},
		{
			name:     "trailing separator on parent",
			parent:   filepath.Join("home", "user") + string(filepath.Separator),
			child:    filepath.Join("home", "user", "documents"),
			expected: true,
		},
		{
			name:     "trailing separator on child",
			parent:   filepath.Join("home", "user"),
			child:    filepath.Join("home", "user", "documents") + string(filepath.Separator),
			expected: true,
		},
		{
			name:     "both have trailing separators",
			parent:   filepath.Join("home", "user") + string(filepath.Separator),
			child:    filepath.Join("home", "user", "documents") + string(filepath.Separator),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsImmediateChild(tt.parent, tt.child)

			if result != tt.expected {
				// Get relative path for debugging
				parent := filepath.Clean(tt.parent)
				child := filepath.Clean(tt.child)
				rel, _ := filepath.Rel(parent, child)

				t.Errorf("IsImmediateChild(%q, %q) = %v, want %v\n"+
					"  Cleaned parent: %q\n"+
					"  Cleaned child:  %q\n"+
					"  Relative path:  %q",
					tt.parent, tt.child, result, tt.expected,
					parent, child, rel)
			}
		})
	}
}
