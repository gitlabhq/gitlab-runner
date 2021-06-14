List of environment variables that are allowed to be passed to tests.
The variables are used in `go_test_no_env` and `go_test_no_env.ps1`.

### allowlist_common

Variables shared between systems. Go-related variables are taken from running `go env`.
Not running `go env` for simplicity and to avoid introducing variables with new go versions that we possibly shouldn't.

### allowlist_unix / allowlist_windows

Only platform-specific env variables.
