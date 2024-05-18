//go:build integration && windows

package process_test

// Cases for Windows that are used in `kill_integration_test.go#TestKiller`.
func testKillerTestCases() map[string]testKillerTestCase {
	return map[string]testKillerTestCase{
		"command not terminated": {
			alreadyStopped:                  false,
			skipTerminate:                   true,
			expectedError:                   "",
			useWindowsLegacyProcessStrategy: true,
			useWindowsJobObject:             false,
		},
		"command terminated": {
			alreadyStopped:                  false,
			skipTerminate:                   false,
			expectedError:                   "exit status 3",
			useWindowsLegacyProcessStrategy: true,
			useWindowsJobObject:             false,
		},
		"command terminated, disable useWindowsLegacyProcessStrategy": {
			alreadyStopped:                  false,
			skipTerminate:                   false,
			expectedError:                   "exit status 3",
			useWindowsLegacyProcessStrategy: false,
			useWindowsJobObject:             false,
		},
		"command already stopped": {
			alreadyStopped:                  true,
			expectedError:                   "exit status 1",
			useWindowsLegacyProcessStrategy: true,
			useWindowsJobObject:             false,
		},
		"command terminated via job object": {
			alreadyStopped:                  false,
			skipTerminate:                   false,
			expectedError:                   "exit status 1",
			useWindowsLegacyProcessStrategy: false,
			useWindowsJobObject:             true,
		},
	}
}
