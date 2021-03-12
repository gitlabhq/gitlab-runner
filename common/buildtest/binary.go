package buildtest

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func MustBuildBinary(entrypoint string, binaryName string) string {
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binaryName, entrypoint)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Executing: %v\n", cmd)

	err := cmd.Run()
	if err != nil {
		panic("Error on executing go build for binary: " + entrypoint)
	}

	return binaryName
}
