// +build gofuzz

package gitlab_ci_yaml_parser

import (
	"log"
	"os"
	"io/ioutil"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func prepareTestFile(fileContent []byte) string {
	file, err := ioutil.TempFile("", "gitlab-ci-yml")
	if err != nil {
		log.Fatalf("create file failed %v", err)
	}
	defer file.Close()

	_, _ = file.Write(fileContent)
	return file.Name()
}

func Fuzz(data []byte) int {
	file := prepareTestFile(data)
	defer os.Remove(file)

	parser := &GitLabCiYamlParser{
		filename: file,
		jobName:  "jobFuzz",
	}

	jobResponse := &common.JobResponse{}
	parser.ParseYaml(jobResponse)
	return 0
}
