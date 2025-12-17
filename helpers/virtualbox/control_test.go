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

func Test_extractHDDInfo(t *testing.T) {
	var tests = []struct {
		name              string
		output            string
		expectedUUIDs     []string
		expectedLocations []string
	}{
		{
			name:              "0 HDDs",
			output:            "",
			expectedUUIDs:     []string{},
			expectedLocations: []string{},
		},
		{
			name:              "1 HDD",
			output:            "UUID:           a8b5aa23-2110-435a-9bdd-8f28e4b840a7\nParent UUID:    base\nLocation:       /home/directory/VirtualBox VMs/vm 1/ vm 1.vdi\nStorage format: VDI\n",
			expectedUUIDs:     []string{"a8b5aa23-2110-435a-9bdd-8f28e4b840a7"},
			expectedLocations: []string{"/home/directory/VirtualBox VMs/vm 1/ vm 1.vdi"},
		},
		{
			name:              "1 HDD with windows style \\r\\n new lines",
			output:            "UUID:           a8b5aa23-2110-435a-9bdd-8f28e4b840a7\r\nParent UUID:    base\r\nLocation:       /home/directory/VirtualBox VMs/vm 1/ vm 1.vdi\r\nStorage format: VDI\r\n",
			expectedUUIDs:     []string{"a8b5aa23-2110-435a-9bdd-8f28e4b840a7"},
			expectedLocations: []string{"/home/directory/VirtualBox VMs/vm 1/ vm 1.vdi"},
		},
		{
			name:              "2 matching HDDs for VM",
			output:            "UUID:           a8b5aa23-2110-435a-9bdd-8f28e4b840a7\nParent UUID:    base\nLocation:       /home/directory/VirtualBox VMs/vm 1/ vm 1.vdi\nStorage format: VDI\n\nUUID:           97bcddfc-b184-4d26-b197-fe287a77fe55\nParent UUID:    base\nLocation:       /home/directory/VirtualBox VMs/vm 2/ vm 2.vdi\nStorage format: VDI\n",
			expectedUUIDs:     []string{"a8b5aa23-2110-435a-9bdd-8f28e4b840a7", "97bcddfc-b184-4d26-b197-fe287a77fe55"},
			expectedLocations: []string{"/home/directory/VirtualBox VMs/vm 1/ vm 1.vdi", "/home/directory/VirtualBox VMs/vm 2/ vm 2.vdi"},
		},
	}
	for _, test := range tests {
		// fixture validation
		assert.Equal(t, len(test.expectedUUIDs), len(test.expectedLocations))

		actualHDDs := extractHDDInfo(test.output)

		assert.Equal(t, len(test.expectedUUIDs), len(actualHDDs))

		for i, hdd := range actualHDDs {
			assert.Equal(t, test.expectedUUIDs[i], hdd[1])
			assert.Equal(t, test.expectedLocations[i], hdd[2])
		}
	}
}
