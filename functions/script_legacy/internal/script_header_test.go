//go:build !integration

package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScriptHeader_Generate_Bash(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	assert.True(t, strings.HasPrefix(result, "#!/bin/bash\n"), "Expected bash shebang, got: %s", result)
	assert.Contains(t, result, trapTerm, "Expected SIGTERM trap")
	assert.Contains(t, result, setPipefailCheck, "Expected pipefail check")
	assert.Contains(t, result, setErrexit, "Expected errexit")
	assert.Contains(t, result, setNoclobber, "Expected noclobber disabled")
	assert.NotContains(t, result, setXtrace, "Should not contain xtrace when debug is disabled")
}

func TestScriptHeader_Generate_BashWithDebug(t *testing.T) {
	header := NewScriptHeader("/bin/bash", true)
	result := header.Generate()

	assert.True(t, strings.HasPrefix(result, "#!/bin/bash\n"), "Expected bash shebang, got: %s", result)
	assert.Contains(t, result, setXtrace, "Expected xtrace when debug enabled")
	assert.Contains(t, result, " -o "+setXtrace, "Expected ' -o xtrace' format")
}

func TestScriptHeader_Generate_Sh(t *testing.T) {
	header := NewScriptHeader("/bin/sh", false)
	result := header.Generate()

	assert.True(t, strings.HasPrefix(result, "#!/bin/sh\n"), "Expected sh shebang, got: %s", result)
	assert.Contains(t, result, setPipefailCheck, "Expected pipefail check")
}

func TestScriptHeader_ContainsPipefailCheck(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	expectedCheck := "if set -o | grep pipefail > /dev/null; then set -o pipefail; fi"
	assert.Contains(t, result, expectedCheck, "Expected conditional pipefail check")
}

func TestScriptHeader_ContainsErrexit(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	assert.Contains(t, result, "set -o errexit", "Expected 'set -o errexit'")
}

func TestScriptHeader_Format(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	assert.True(t, strings.HasPrefix(result, "#!/bin/bash\n\n"), "Expected shebang followed by blank line")
	assert.True(t, strings.HasSuffix(result, "\n\n"), "Expected to end with double newline")
}

func TestScriptHeader_ContainsTrapTerm(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	assert.Contains(t, result, "trap exit 1 TERM", "Expected SIGTERM trap for clean cancellation")
}

func TestScriptHeader_ContainsNoclobber(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	assert.Contains(t, result, "set +o noclobber", "Expected noclobber disabled for file overwrite compatibility")
}

func TestScriptHeader_SecurityFeatures(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	securityFeatures := []struct {
		feature string
		desc    string
	}{
		{"trap exit 1 TERM", "Prevents script dump on cancellation"},
		{"set -o errexit", "Exit on error"},
		{"set +o noclobber", "Allow file overwrites"},
	}

	for _, sf := range securityFeatures {
		assert.Contains(t, result, sf.feature, "Missing security feature: %s", sf.desc)
	}
}

func TestScriptHeader_OrderOfOptions(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	// Verify correct order: shebang -> trap -> pipefail -> errexit -> noclobber
	trapIdx := strings.Index(result, "trap exit 1 TERM")
	pipefailIdx := strings.Index(result, "if set -o | grep pipefail")
	errexitIdx := strings.Index(result, "set -o errexit")
	noclobberIdx := strings.Index(result, "set +o noclobber")

	assert.NotEqual(t, -1, trapIdx, "Missing trap option")
	assert.NotEqual(t, -1, pipefailIdx, "Missing pipefail option")
	assert.NotEqual(t, -1, errexitIdx, "Missing errexit option")
	assert.NotEqual(t, -1, noclobberIdx, "Missing noclobber option")

	assert.Less(t, trapIdx, pipefailIdx, "trap should come before pipefail")
	assert.Less(t, pipefailIdx, errexitIdx, "pipefail should come before errexit")
	assert.Less(t, errexitIdx, noclobberIdx, "errexit should come before noclobber")
}
