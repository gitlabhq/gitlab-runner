//go:build !integration

package shells

import (
	"math/rand/v2"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var procRegSetValueEx = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegSetValueExW")

// setRegistryValue writes a value of any type and length. registry.Key cannot write a
// REG_DWORD that is not 4 bytes long, and the windows package does not export RegSetValueEx.
func setRegistryValue(key registry.Key, name string, valueType uint32, data []byte) error {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}

	ret, _, _ := procRegSetValueEx.Call(
		uintptr(key),
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(valueType),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
	)
	if ret != 0 {
		return syscall.Errno(ret)
	}

	return nil
}

// TestPowershell_CPUInfoMessage_UnconvertibleRegistryValue does not depend on the registry of
// the machine running it. It builds a key with a valid ProcessorNameString next to a value that
// Get-ItemProperty cannot convert, which is what some Intel desktop CPUs have in
// "Patch Configuration Used", and points the script at it.
func TestPowershell_CPUInfoMessage_UnconvertibleRegistryValue(t *testing.T) {
	pwsh, err := exec.LookPath(SNPwsh)
	if err != nil {
		t.Skip("requires pwsh")
	}
	require.Contains(t, pwshCPUInfoScript, pwshCPUKeyLiteral)

	const model = "Fixture(R) CPU @ 1.00GHz"

	// Get-ItemProperty reads a REG_DWORD as an Int32 only when it holds exactly 4 bytes, and
	// fails to convert it otherwise. Affected machines hold 1 byte.
	tests := map[string][]byte{
		"1 byte":  make([]byte, 1),
		"8 bytes": make([]byte, 8),
	}

	for tn, data := range tests {
		t.Run(tn, func(t *testing.T) {
			keyPath := `Software\gitlab-runner-test-` + strconv.FormatUint(rand.Uint64(), 16)
			key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
			require.NoError(t, err)
			t.Cleanup(func() {
				assert.NoError(t, key.Close())
				assert.NoError(t, registry.DeleteKey(registry.CURRENT_USER, keyPath))
			})

			require.NoError(t, key.SetStringValue("ProcessorNameString", model))
			require.NoError(t, setRegistryValue(key, "Patch Configuration Used", registry.DWORD, data))

			script := strings.Replace(pwshCPUInfoScript, pwshCPUKeyLiteral, `'HKCU:\`+keyPath+`'`, 1)

			assert.Equal(t, "Running on CPU: "+model, runPwshCPUInfoScript(t, pwsh, script))
		})
	}
}
