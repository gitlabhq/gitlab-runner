package common

import (
	"os"
	"path"
	"runtime"
	"strings"
)

const repoRemoteURL = "https://gitlab.com/gitlab-org/gitlab-test.git"
const repoSHA = "6907208d755b60ebeacb2e9dfea74c92c3449a1f"
const repoBeforeSHA = "c347ca2e140aa667b968e51ed0ffe055501fe4f4"
const repoRefName = "master"

func GetSuccessfulBuild() (GetBuildResponse, error) {
	return getLocalBuildResponse("echo Hello World")
}

func GetRemoteSuccessfulBuild() (GetBuildResponse, error) {
	return getRemoteBuildResponse("echo Hello World")
}

func GetFailedBuild() (GetBuildResponse, error) {
	return getLocalBuildResponse("exit 1")
}

func GetRemoteFailedBuild() (GetBuildResponse, error) {
	return getRemoteBuildResponse("exit 1")
}

func GetLongRunningBuild() (GetBuildResponse, error) {
	return getLocalBuildResponse("sleep 3600")
}

func GetRemoteLongRunningBuild() (GetBuildResponse, error) {
	return getRemoteBuildResponse("sleep 3600")
}

func getRemoteBuildResponse(commands string) (response GetBuildResponse, err error) {
	response = GetBuildResponse{
		Commands:  commands,
		RepoURL:   repoRemoteURL,
		Sha:       repoSHA,
		BeforeSha: repoBeforeSHA,
		RefName:   repoRefName,
	}

	return
}

func getLocalBuildResponse(commands string) (response GetBuildResponse, err error) {
	localRepoURL, err := getLocalRepoURL()
	if err != nil {
		return
	}

	response = GetBuildResponse{
		Commands:  commands,
		RepoURL:   localRepoURL,
		Sha:       repoSHA,
		BeforeSha: repoBeforeSHA,
		RefName:   repoRefName,
	}

	return
}

func getLocalRepoURL() (string, error) {
	_, filename, _, _ := runtime.Caller(0)

	directory := path.Dir(filename)
	if strings.Contains(directory, "_test/_obj_test") {
		pwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		directory = pwd
	}

	localRepoURL := path.Clean(directory + "/../tmp/gitlab-test/.git")

	_, err := os.Stat(localRepoURL)
	if err != nil {
		return "", err
	}

	return localRepoURL, nil
}
