package common

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Native steps execution is enabled if:
// - the job uses the run keyword.
// - the executor supports native steps.
// - we are not running on windows
func (b *Build) UseNativeSteps() bool {
	return b.JobResponse.Run != "" &&
		b.ExecutorFeatures.NativeStepsIntegration &&
		runtime.GOOS != "windows"
}

const (
	defaultStepRunnerImage   = "registry.gitlab.com/gitlab-org/step-runner"
	stepRunnerModule         = "gitlab.com/gitlab-org/step-runner"
	defaultStepRunnerVersion = "0.16" // only necessary in unit tests in Go versions < 1.24
)

// getModuleDependencyVersion returns the version of the specific module dependency against which the running binary was
// compiled, with the leading "v" removed. Note that until 1.24 lands, the list of dependencies is empty when running
// tests that are not in the main package. The defaultValue is only necessary for that case. See:
// https://github.com/golang/go/commit/d79e6bec6389dfeeec84a64f283055090615bad1
func getModuleDependencyVersion(modulePath, defaultValue string) string {
	bInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return defaultValue
	}

	for _, d := range bInfo.Deps {
		if d.Path == modulePath {
			return strings.TrimLeft(d.Version, "v")
		}
	}

	return defaultValue
}

// GetStepRunnerImage returns the step-runner image to use to inject the step-runner binary into the build environment.
// The image version should ideally match the version of the step-runner library against which runner was built, but as
// an escape hatch the image and version can be specified via Variables.
func (r *RunnerSettings) GetStepRunnerImage() string {
	if strings.TrimSpace(r.StepRunnerImage) != "" {
		return r.StepRunnerImage
	}

	stepRunnerVersion := getMajorMinorVersion(getModuleDependencyVersion(stepRunnerModule, defaultStepRunnerVersion))

	return fmt.Sprintf("%s:%s", defaultStepRunnerImage, stepRunnerVersion)
}

func getMajorMinorVersion(version string) string {
	versions := strings.Split(version, ".")
	switch len(versions) {
	case 1:
		return versions[0]
	default:
		return strings.Join(versions[0:2], ".")
	}
}
