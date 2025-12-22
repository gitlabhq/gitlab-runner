//go:build !integration

package internal

import (
	"strings"
	"testing"
)

func TestScriptHeader_Generate_Bash(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	if !strings.HasPrefix(result, "#!/bin/bash\n") {
		t.Errorf("Expected bash shebang, got: %s", result)
	}

	if !strings.Contains(result, trapTerm) {
		t.Errorf("Expected SIGTERM trap")
	}

	if !strings.Contains(result, setPipefailCheck) {
		t.Errorf("Expected pipefail check")
	}

	if !strings.Contains(result, setErrexit) {
		t.Errorf("Expected errexit")
	}

	if !strings.Contains(result, setNoclobber) {
		t.Errorf("Expected noclobber disabled")
	}

	if strings.Contains(result, setXtrace) {
		t.Errorf("Should not contain xtrace when debug is disabled")
	}
}

func TestScriptHeader_Generate_BashWithDebug(t *testing.T) {
	header := NewScriptHeader("/bin/bash", true)
	result := header.Generate()

	if !strings.HasPrefix(result, "#!/bin/bash\n") {
		t.Errorf("Expected bash shebang, got: %s", result)
	}

	if !strings.Contains(result, setXtrace) {
		t.Errorf("Expected xtrace when debug enabled")
	}

	if !strings.Contains(result, " -o "+setXtrace) {
		t.Errorf("Expected ' -o xtrace' format")
	}
}

func TestScriptHeader_Generate_Sh(t *testing.T) {
	header := NewScriptHeader("/bin/sh", false)
	result := header.Generate()

	if !strings.HasPrefix(result, "#!/bin/sh\n") {
		t.Errorf("Expected sh shebang, got: %s", result)
	}

	if !strings.Contains(result, setPipefailCheck) {
		t.Errorf("Expected pipefail check")
	}
}

func TestScriptHeader_ContainsPipefailCheck(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	// Pipefail should be conditional for sh compatibility
	expectedCheck := "if set -o | grep pipefail > /dev/null; then set -o pipefail; fi"
	if !strings.Contains(result, expectedCheck) {
		t.Errorf("Expected conditional pipefail check")
	}
}

func TestScriptHeader_ContainsErrexit(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	if !strings.Contains(result, "set -o errexit") {
		t.Errorf("Expected 'set -o errexit'")
	}
}

func TestScriptHeader_Format(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	if !strings.HasPrefix(result, "#!/bin/bash\n\n") {
		t.Errorf("Expected shebang followed by blank line")
	}

	if !strings.HasSuffix(result, "\n\n") {
		t.Errorf("Expected to end with double newline")
	}
}

func TestScriptHeader_ContainsTrapTerm(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	if !strings.Contains(result, "trap exit 1 TERM") {
		t.Errorf("Expected SIGTERM trap for clean cancellation")
	}
}

func TestScriptHeader_ContainsNoclobber(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	if !strings.Contains(result, "set +o noclobber") {
		t.Errorf("Expected noclobber disabled for file overwrite compatibility")
	}
}

func TestScriptHeader_SecurityFeatures(t *testing.T) {
	header := NewScriptHeader("/bin/bash", false)
	result := header.Generate()

	securityFeatures := []string{
		"trap exit 1 TERM", // Prevents script dump on cancellation
		"set -o errexit",   // Exit on error
		"set +o noclobber", // Allow file overwrites
	}

	for _, feature := range securityFeatures {
		if !strings.Contains(result, feature) {
			t.Errorf("Missing security feature: %s", feature)
		}
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

	if trapIdx == -1 || pipefailIdx == -1 || errexitIdx == -1 || noclobberIdx == -1 {
		t.Errorf("Missing expected options")
		return
	}

	if trapIdx > pipefailIdx {
		t.Errorf("trap should come before pipefail")
	}
	if pipefailIdx > errexitIdx {
		t.Errorf("pipefail should come before errexit")
	}
	if errexitIdx > noclobberIdx {
		t.Errorf("errexit should come before noclobber")
	}
}
