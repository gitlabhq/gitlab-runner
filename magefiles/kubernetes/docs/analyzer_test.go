package docs

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAnalyzerFilter(testFile string) func(fileInfo fs.FileInfo) bool {
	return func(fileInfo fs.FileInfo) bool {
		return testFile == fileInfo.Name()
	}
}

var expectedGroup = PermissionsGroup{
	"pods": []verb{{
		Verb:        "get",
		ConfigFlags: make([]configFlag, 0),
	}},
}

func TestParsePermissionsPointerStructField(t *testing.T) {
	grp, err := parsePermissions("testdata/", testAnalyzerFilter("kubernetes_analyzer_api_pointer_call.go"))
	assert.ErrorContains(t, err, "13:9 Missing")
	assert.Equal(t, expectedGroup, grp)
}

func TestParsePermissionsNonPointerStructField(t *testing.T) {
	grp, err := parsePermissions("testdata/", testAnalyzerFilter("kubernetes_analyzer_api_nonpointer_call.go"))
	require.ErrorContains(t, err, "13:9 Missing")
	assert.Equal(t, expectedGroup, grp)
}

func TestParsePermissionsUnnamedFieldCall(t *testing.T) {
	grp, err := parsePermissions("testdata/", testAnalyzerFilter("kubernetes_analyzer_api_unnamed_field.go"))
	require.ErrorContains(t, err, "13:9 Missing")
	assert.Equal(t, expectedGroup, grp)
}

func TestParsePermissionsDeclaration(t *testing.T) {
	grp, err := parsePermissions("testdata/", testAnalyzerFilter("kubernetes_analyzer_api_declaration.go"))
	require.ErrorContains(t, err, "10:9 Missing")
	assert.Equal(t, expectedGroup, grp)
}

func TestParsePermissionsDeclarationReassigned(t *testing.T) {
	grp, err := parsePermissions("testdata/", testAnalyzerFilter("kubernetes_analyzer_api_declaration_reassigned.go"))
	require.ErrorContains(t, err, "12:9 Missing")
	assert.Equal(t, expectedGroup, grp)
}

func TestParsePermissionsFnArg(t *testing.T) {
	grp, err := parsePermissions("testdata/", testAnalyzerFilter("kubernetes_analyzer_api_fn_arg.go"))
	require.ErrorContains(t, err, "9:9 Missing")
	assert.Equal(t, expectedGroup, grp)
}

func TestKubernetes(t *testing.T) {
	_, err := parsePermissions("../../../executors/kubernetes", filterTestFiles)
	require.NoError(t, err)
}
