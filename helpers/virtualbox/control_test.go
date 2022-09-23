//go:build !integration

package virtualbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapshotNameRegex(t *testing.T) {
	var tests = []struct {
		output         string
		snapshotName   string
		expectedToFind bool
	}{
		{`SnapshotName="v1"`, "v1", true},
		{`SnapshotName="gitlabrunner"`, "gitlabrunner", true},
		{"SnapshotName=\"gitlabrunner\"\nSnapshotUUID=\"UUID\"\n", "gitlabrunner", true},
		{"SnapshotName=\"gitlabrunner\"\nSnapshotUUID=\"UUID\"\n", "notpresent", false},
		// Windows style \r\n new lines
		{"SnapshotName=\"gitlabrunner\"\r\nSnapshotUUID=\"UUID\"\r\n", "gitlabrunner", true},
	}
	for _, test := range tests {
		assert.Equal(t, test.expectedToFind, matchSnapshotName(test.snapshotName, test.output))
	}
}

func TestCurrentSnapshotNameRegex(t *testing.T) {
	var tests = []struct {
		output               string
		expectedSnapshotName string
		expectedToFind       bool
	}{
		{`CurrentSnapshotName="v1"`, "v1", true},
		{`CurrentSnapshotName="gitlabrunner"`, "gitlabrunner", true},
		{"CurrentSnapshotName=\"gitlabrunner\"\nCurrentSnapshotUUID=\"UUID\"\n", "gitlabrunner", true},
		{"CurrentSnapshotName=\"gitlabrunner\"\nCurrentSnapshotUUID=\"UUID\"\n", "notpresent", false},
		// Windows style \r\n new lines
		{"CurrentSnapshotName=\"gitlabrunner\"\r\nCurrentSnapshotUUID=\"UUID\"\r\n", "gitlabrunner", true},
	}
	for _, test := range tests {
		actual := matchCurrentSnapshotName(test.output)

		if test.expectedToFind {
			assert.NotNil(t, actual)
			assert.Equal(t, test.expectedSnapshotName, actual[1])
			return
		}

		assert.NotEqual(t, test.expectedSnapshotName, actual[1])
	}
}
