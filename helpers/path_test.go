//go:build !integration

package helpers

import "testing"

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
