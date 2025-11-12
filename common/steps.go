package common

import (
	"runtime"
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
