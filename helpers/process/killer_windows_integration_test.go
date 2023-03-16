//go:build integration && windows

package process_test

// Cases for Windows that are used in `filler.go#TestKiller`.
func testKillerTestCases() map[string]testKillerTestCase {
	return map[string]testKillerTestCase{
		"command not terminated": {
			alreadyStopped:                  false,
			skipTerminate:                   true,
			expectedError:                   "",
			useWindowsLegacyProcessStrategy: true,
		},
		"command terminated": {
			alreadyStopped:                  false,
			skipTerminate:                   false,
			expectedError:                   "exit status 1",
			useWindowsLegacyProcessStrategy: true,
		},
		"command terminated, disable useWindowsLegacyProcessStrategy": {
			alreadyStopped:                  false,
			skipTerminate:                   false,
			expectedError:                   "",
			useWindowsLegacyProcessStrategy: false,
		},
		"command already stopped": {
			alreadyStopped:                  true,
			expectedError:                   "exit status 1",
			useWindowsLegacyProcessStrategy: true,
		},
	}
}
