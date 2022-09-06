//nolint:lll
//go:build integration && (aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris)

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
