package build

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const (
	skopeoImage = "quay.io/skopeo/stable:v1.12.0"
)

var skopeoErrorMessageRegex = regexp.MustCompile(`time=".+"\slevel=\w+\smsg="(?P<message>.+)"`)

type ResourceChecker interface {
	Exists() error
}

func NewResourceChecker(c Component) ResourceChecker {
	switch c.Type() {
	case TypeDockerImage:
		return newDockerImageChecker(c.Value())
	case TypeFile:
		return newFileChecker(c.Value())
	case TypeDockerImageArchive:
		return newFileChecker(c.Value())
	case TypeOSBinary:
		return newBinaryPathChecker(c.Value())
	case TypeMacOSPackage:
		return newBinaryPathChecker(c.Value())
	default:
		return unknownResourceChecker{}
	}
}

type unknownResourceChecker struct {
}

func (unknownResourceChecker) Exists() error {
	return errors.New("unknown")
}

type fileChecker struct {
	file string
}

func newFileChecker(f string) fileChecker {
	return fileChecker{file: f}
}

func (f fileChecker) Exists() error {
	_, err := os.Stat(f.file)
	if err != nil {
		substr := fmt.Sprintf("stat %s: ", f.file)
		if strings.HasPrefix(err.Error(), substr) {
			return errors.New(strings.Replace(err.Error(), substr, "", 1))
		}
	}

	return err
}

type dockerImageChecker struct {
	image string
}

func newDockerImageChecker(image string) *dockerImageChecker {
	return &dockerImageChecker{image: image}
}

// dockerInspectArgs builds the skopeo/docker inspect arguments for image,
// attaching registry credentials only when they match the image's own
// registry. Each case must only read its own registry's credentials -
// crossing them (eg. sending GitLab.com credentials to Docker Hub) causes
// every check against the other registry to fail authentication.
func dockerInspectArgs(image string) []string {
	args := []string{"inspect", "--raw", "--no-tags"}

	switch lower := strings.ToLower(image); {
	case strings.HasPrefix(lower, "registry.gitlab.com"):
		// This is mostly for the security fork, to be able to query images from the security repos
		if user, pass := os.Getenv("CI_REGISTRY_USER"), os.Getenv("CI_REGISTRY_PASSWORD"); user != "" && pass != "" {
			args = append(args, "--username", user, "--password", pass)
		}
	case strings.HasPrefix(lower, "registry.hub.docker.com"):
		// Docker Hub's anonymous pull limit (100/6h per IP) is easily
		// exceeded by checking the full release tag matrix unauthenticated.
		// Reuse the credentials the release jobs already use to push these
		// same images.
		if user, pass := os.Getenv("DOCKER_HUB_USER"), os.Getenv("DOCKER_HUB_PASSWORD"); user != "" && pass != "" {
			args = append(args, "--username", user, "--password", pass)
		}
	}

	return args
}

func (d *dockerImageChecker) Exists() error {
	// the results of this function can be cached but there's no need atm
	args := append(dockerInspectArgs(d.image), "docker://"+d.image)
	command := "skopeo"
	_, err := exec.LookPath(command)
	if err != nil {
		command = "docker"
		args = append([]string{"run", "--rm", skopeoImage}, args...)
	}

	out, err := exec.Command(command, args...).CombinedOutput()
	if err == nil {
		return nil
	}

	if strings.Contains(string(out), "manifest unknown") {
		return errors.New("manifest unknown")
	}

	// parse skopeo error message such as
	// time="2023-10-10T22:45:14+03:00" level=fatal msg="Error parsing image name \"docker://gitlab-runner:bleeding\":
	// reading manifest bleeding in docker.io/library/gitlab-runner: requested access to the resource is denied"
	matches := skopeoErrorMessageRegex.FindStringSubmatch(string(out))
	if len(matches) == 0 {
		return errors.New(string(out))
	}

	errMessage := matches[skopeoErrorMessageRegex.SubexpIndex("message")]
	return errors.New(errMessage)
}

type binaryPathChecker struct {
	bin string
}

func newBinaryPathChecker(bin string) *binaryPathChecker {
	return &binaryPathChecker{bin: bin}
}

func (b *binaryPathChecker) Exists() error {
	_, err := exec.LookPath(b.bin)
	return err
}
