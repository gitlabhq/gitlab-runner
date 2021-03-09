// +build darwin dragonfly freebsd linux netbsd openbsd

package process_test

// Cases for UNIX systems that are used in `killer_test.go#TestKiller`.
func testKillerTestCases() map[string]testKillerTestCase {
	return map[string]testKillerTestCase{
		"command terminated": {
			alreadyStopped: false,
			skipTerminate:  true,
			expectedError:  "",
		},
		"command not terminated": {
			alreadyStopped: false,
			skipTerminate:  false,
			expectedError:  "exit status 1",
		},
		"command already stopped": {
			alreadyStopped: true,
			expectedError:  "signal: killed",
		},
	}
}
