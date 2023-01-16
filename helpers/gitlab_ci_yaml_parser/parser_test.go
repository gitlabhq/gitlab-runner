//go:build !integration

package gitlab_ci_yaml_parser

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var testFile1 = `
image: global:image

job1:
  stage: test
  script:
  - line 1
  - line 2
  image: job1:image
  services:
  - service:1
  - service:2

job2:
  script: test

job3:
  stage: a

job4:
  script: job4
  image:
    name: alpine
    entrypoint: ["/bin/sh"]
  services:
  - name: service:1
    command: ["sleep", "30"]
    alias: service-1
  - name: service:2
    entrypoint: ["/bin/sh"]
    alias: service-2
`

var testFile2 = `
image:
  name: global:image
  entrypoint: [/bin/sh]

services:
- name: service:1
  command: ["sleep", "30"]
  alias: service-1
- name: service:2
  entrypoint: [/bin/sh]
  alias: service-2

job1:
  script: job1

job2:
  script: job1
  image: job2:image
  services:
  - service:1
  - service:2
`

var testFile3 = `
image:
  name: global:image
  entrypoint: [/bin/sh]

job1:
  script: job1
  services:
  - name: service:1
    command: ["sleep", "30"]
    alias: service-1 service-1-alias
`

var testFile4 = `
image:
  name: global:image
  entrypoint: [/bin/sh]

job1:
  script: job1
  services:
  - name: service:1
    command: ["sleep", "30"]
    alias: service-1 42
`

func prepareTestFile(t *testing.T, fileContent string) string {
	file, err := os.CreateTemp("", "gitlab-ci-yml")
	require.NoError(t, err)
	defer file.Close()

	_, _ = file.WriteString(fileContent)
	return file.Name()
}

func getJobResponse(t *testing.T, fileContent, jobName string, expectingError string) *common.JobResponse {
	file := prepareTestFile(t, fileContent)
	defer os.Remove(file)

	parser := &GitLabCiYamlParser{
		filename: file,
		jobName:  jobName,
	}

	jobResponse := &common.JobResponse{}
	err := parser.ParseYaml(jobResponse)
	if expectingError != "" {
		assert.Error(t, err)
		assert.Equal(t, err.Error(), expectingError)
	} else {
		assert.NoError(t, err)
	}

	return jobResponse
}

func TestFileParsing(t *testing.T) {
	// file1 - job1
	jobResponse := getJobResponse(t, testFile1, "job1", "")
	require.Len(t, jobResponse.Steps, 2)
	assert.Contains(t, jobResponse.Steps[0].Script, "line 1")
	assert.Contains(t, jobResponse.Steps[0].Script, "line 2")
	assert.Equal(t, "test", jobResponse.JobInfo.Stage)
	assert.Equal(t, "job1:image", jobResponse.Image.Name)
	require.Len(t, jobResponse.Services, 2)
	assert.Equal(t, "service:1", jobResponse.Services[0].Name)
	assert.Empty(t, jobResponse.Services[0].Alias)
	assert.Empty(t, jobResponse.Services[0].Command)
	assert.Empty(t, jobResponse.Services[0].Entrypoint)
	assert.Equal(t, "service:2", jobResponse.Services[1].Name)
	assert.Empty(t, jobResponse.Services[1].Alias)
	assert.Empty(t, jobResponse.Services[1].Command)
	assert.Empty(t, jobResponse.Services[1].Entrypoint)

	// file1 - job2
	jobResponse = getJobResponse(t, testFile1, "job2", "")
	require.Len(t, jobResponse.Steps, 2)
	assert.Contains(t, jobResponse.Steps[0].Script, "test")
	assert.Equal(t, "global:image", jobResponse.Image.Name)

	// file1 - job3
	_ = getJobResponse(t, testFile1, "job3", "missing 'script' for job")

	// file1 - job4
	jobResponse = getJobResponse(t, testFile1, "job4", "")
	assert.Equal(t, "alpine", jobResponse.Image.Name)
	assert.Equal(t, []string{"/bin/sh"}, jobResponse.Image.Entrypoint)
	require.Len(t, jobResponse.Services, 2)
	assert.Equal(t, "service:1", jobResponse.Services[0].Name)
	assert.Equal(t, "service-1", jobResponse.Services[0].Alias)
	assert.Equal(t, []string{"sleep", "30"}, jobResponse.Services[0].Command)
	assert.Empty(t, jobResponse.Services[0].Entrypoint)
	assert.Equal(t, "service:2", jobResponse.Services[1].Name)
	assert.Equal(t, "service-2", jobResponse.Services[1].Alias)
	assert.Empty(t, jobResponse.Services[1].Command)
	assert.Equal(t, []string{"/bin/sh"}, jobResponse.Services[1].Entrypoint)

	// file2 - job1
	jobResponse = getJobResponse(t, testFile2, "job1", "")
	assert.Equal(t, "global:image", jobResponse.Image.Name)
	assert.Equal(t, []string{"/bin/sh"}, jobResponse.Image.Entrypoint)
	require.Len(t, jobResponse.Services, 2)
	assert.Equal(t, "service:1", jobResponse.Services[0].Name)
	assert.Equal(t, "service-1", jobResponse.Services[0].Alias)
	assert.Equal(t, []string{"sleep", "30"}, jobResponse.Services[0].Command)
	assert.Empty(t, jobResponse.Services[0].Entrypoint)
	assert.Equal(t, "service:2", jobResponse.Services[1].Name)
	assert.Equal(t, "service-2", jobResponse.Services[1].Alias)
	assert.Empty(t, jobResponse.Services[1].Command)
	assert.Equal(t, []string{"/bin/sh"}, jobResponse.Services[1].Entrypoint)

	// file2 - job2
	jobResponse = getJobResponse(t, testFile2, "job2", "")
	assert.Equal(t, "job2:image", jobResponse.Image.Name)
	assert.Empty(t, jobResponse.Image.Entrypoint)
	require.Len(t, jobResponse.Services, 2)
	assert.Equal(t, "service:1", jobResponse.Services[0].Name)
	assert.Empty(t, jobResponse.Services[0].Alias)
	assert.Empty(t, jobResponse.Services[0].Command)
	assert.Empty(t, jobResponse.Services[0].Entrypoint)
	assert.Equal(t, "service:2", jobResponse.Services[1].Name)
	assert.Empty(t, jobResponse.Services[1].Alias)
	assert.Empty(t, jobResponse.Services[1].Command)
	assert.Empty(t, jobResponse.Services[1].Entrypoint)

	// file3 - job1
	jobResponse = getJobResponse(t, testFile3, "job1", "")
	assert.Equal(t, "global:image", jobResponse.Image.Name)
	require.Len(t, jobResponse.Services, 1)
	assert.Equal(t, "service:1", jobResponse.Services[0].Name)
	assert.Equal(t, "service-1 service-1-alias", jobResponse.Services[0].Alias)
	assert.Equal(t, []string{"service-1", "service-1-alias"}, jobResponse.Services[0].Aliases())

	// file4 - job1
	jobResponse = getJobResponse(t, testFile4, "job1", "")
	assert.Equal(t, "global:image", jobResponse.Image.Name)
	require.Len(t, jobResponse.Services, 1)
	assert.Equal(t, "service:1", jobResponse.Services[0].Name)
	assert.Equal(t, "service-1 42", jobResponse.Services[0].Alias)
	assert.Equal(t, []string{"service-1", "42"}, jobResponse.Services[0].Aliases())
}
