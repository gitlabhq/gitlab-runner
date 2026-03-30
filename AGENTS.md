# GitLab Runner — AI Agent Instructions

This file provides context for AI agents operating on this repository.
All agent reasoning, analysis, and action plans should be written to stdout.
Do not post comments to issues or merge requests during the fix process.

## Codebase overview

**Language:** Go | **Min version:** see `go.mod` | **Default branch:** `main`

| Package | Purpose |
|---|---|
| `executors/` | Executor implementations: `docker`, `kubernetes`, `shell`, `ssh`, `instance`, `custom` |
| `commands/` | CLI entry points: `run`, `register`, `exec`, `artifacts-downloader`, etc. |
| `network/` | GitLab API client: job polling, artifact upload, trace streaming |
| `helpers/` | Shared utilities, retry logic, process management, file operations |
| `shells/` | Shell script generation: Bash, PowerShell, CMD |
| `cache/` | Cache backends: S3, GCS, Azure |
| `common/` | Core types: `Config`, `Runner`, `Build`, `JobResponse`, `Network`, `Executor` |
| `referees/` | Metric collection during job execution |

## Deprecated features — do not invest in fixes

- **docker+machine executor** — deprecated GitLab 17.5, removal GitLab 20.0 (May 2027)
- **`gitlab-runner exec` command** — deprecated, scheduled for removal

When asked to fix a bug in these features: log the deprecation status and
migration path to stdout, then exit. Do not create branches or MRs.

## Coding standards

- Follow existing patterns in the file you are editing
- Error wrapping: `fmt.Errorf("context: %w", err)` not `errors.Wrap`
- Logging: use the structured logger (`logrus`) already imported in the file
- Tests: table-driven tests using `testify/require` and `testify/assert`
- Mocks: generated with `mockery` — check `//go:generate` directives before writing manual mocks
- Do not modify `go.mod` / `go.sum` unless the fix genuinely requires a new dependency
- Do not refactor code unrelated to the bug being fixed

### Fix the root cause, not a downstream symptom

Before writing any code, identify the specific function, call site, and ordering where the
invariant breaks. State it in your commit message. Do not patch a proxy or downstream layer
when the root cause is accessible — downstream patches leave the original bug in place and
create two code paths to maintain.

### Fix the general case, not just the reported input

When a bug is reported for one specific value (e.g. a variable name with a dash, or one
specific `GIT_STRATEGY`), examine the full input domain and fix the general case. Patching
only the reported example creates false confidence and deferred failures for inputs in the
same class. Your fix must be at least as broad as the problem domain.

### Reuse existing helpers — search before adding

Before writing a new helper function, search the package for an existing one:
`grep -rn "concept\|related_term" ./package/`. If a correct helper already exists, use it.
If you do introduce a new function, state in a comment why the existing helpers were
insufficient. Duplicating logic is a maintenance liability and a code smell.

### Match the nil-vs-zero-value return contract

In Go, `nil` and `&ZeroStruct{}` are semantically distinct. Before adding a return statement
to an existing function, audit every other return site and confirm the contract: does the
caller distinguish error from success via a nil check or by inspecting fields? Returning a
non-nil zero-value struct where callers expect nil on error can trigger downstream panics
(e.g. `if result.ID == 0 { logrus.Panicln(...) }`). When in doubt, return `nil` on failure.

## Verification

After making changes, always run these in order before pushing. All must pass:
- `make tools` — installs golangci-lint and other dev tools into `.tmp/bin/` (required before linting)
- `make development_setup` — sets up local git repo fixtures needed by some tests (idempotent)
- `go build ./...` — must compile clean
- `go vet ./...` — must pass clean
- `go test -race ./... -count=1 -timeout 30m` — fix any failures your changes introduced; `-race` adds ~10× overhead so the timeout must be generous
- `make lint` — runs golangci-lint via the Makefile (version is pinned in `GOLANGLINT_VERSION` in the Makefile; always use `make lint` rather than calling the binary directly so the pinned version is used)

Do not log `CI_JOB_TOKEN`, API tokens, or any secret value to stdout. If you need to
diagnose an authentication failure, log the HTTP status code and response body only.

Some tests require a live Docker daemon, Kubernetes cluster, or real GitLab instance
and will fail in CI. These are expected — log them explicitly and continue.
Do not treat pre-existing infrastructure-dependent failures as blockers.

## Commit and branch conventions

- Branch name: `fix/issue-{IID}-short-kebab-description`
- Commit message: `fix: imperative description (closes #{IID})`
- MR description must explain root cause and the fix, not just what changed

## Bug triage — when to stop

Stop and log reasoning without creating an MR when:
- The bug affects a deprecated executor or command (see above)
- The root cause cannot be determined from available context
- The fix would require changes across more than 5 files or touches core architecture
- The issue has a `security` label — these require human review
- The issue has a `customer` or `priority::1` / `priority::2` label — flag for @adebayo_a

## Focus discipline

**Do not fix unrelated CI pipeline failures.** If the repository's CI pipeline is
failing for a reason unrelated to the issue you were assigned, note it in the MR
description and continue with the assigned fix. Do not open branches to repair CI
unless the failing pipeline was explicitly introduced by your own changes.

## Patterns from past fixes

Use this section during research to recognise familiar bug classes before diving into code.

### Context handling
- Always return the deadline context error, not the parent context error.
  In retry/backoff loops, `ctx.Err()` on the wrong context is a recurring mistake.
- Replace `time.After` in loops with `time.NewTimer` + explicit `Stop()`. `time.After`
  leaks timers until they fire; in retry loops this causes unnecessary allocations
  and delayed cancellation. (MR !6064)

### Data races
- Shared state accessed from goroutines must be protected with a mutex or communicated
  via channels. The WebSocket tunnel and the runner fleet scheduler are known areas
  where races have occurred. Always run `go test -race` before pushing. (MR !6237)

### String encoding and filenames
- File names passed to archive headers (gzip, zip, tar) must be sanitised before use.
  Non-ASCII characters cause latin-1 encoding errors in gzip headers. Use the existing
  sanitisation helper in `helpers/` rather than passing raw paths. (MR !6487)

### Configuration changes
- New config fields that change existing behaviour must default to preserving the old
  behaviour. Never change the meaning of an existing field's zero value — that is a
  breaking change. (MR !6081)

### S3 / cache errors
- S3 403 errors often mean missing session token, not bad credentials. Check whether
  the credentials chain includes a session token and ensure it is forwarded. (MR !6376, !6472)

### Nil guards
- `filepath.Walk` can pass a nil `FileInfo` when it encounters a permission error.
  Always nil-check `FileInfo` before accessing its methods. (MR !6050)

### PowerShell variable name escaping
- When generating PowerShell variable references, any name containing characters
  outside `[a-zA-Z0-9_]` (dashes, dots, spaces, etc.) must use `${name}` syntax.
  Bare `$name` is invalid for such names — PowerShell parses `$MY-VAR` as
  `($MY) - (VAR)`, producing a syntax error or wrong value.
- Use a regex guard `[^a-zA-Z0-9_]` — not just `strings.Contains(name, "-")`.
  Dots, spaces, and other special characters trigger the same problem.

### Symlink traversal
- `filepath.Walk` does not follow directory symlinks. When walking artifact paths,
  glob results, or any user-specified path, check whether the walk root is a symlink
  and resolve it with `filepath.EvalSymlinks` before calling Walk.
- **Always include cycle detection**: use a `visited map[string]struct{}` keyed on
  real (resolved) paths. A circular symlink without detection causes an infinite loop.
  Failing to detect cycles is a correctness bug, not a performance concern.
- **Always add a cycle termination test**: name it `Test<Function>_CycleDoesNotHang`,
  construct an actual circular symlink in a temp dir, and assert the function returns
  within a reasonable deadline. An untested cycle path is a production denial-of-service
  risk in multi-tenant CI environments.

### Feature flags and git strategy completeness
- When a feature flag controls behaviour that runs in a switch over `GetGitStrategy()`,
  check that every strategy branch (`GitClone`, `GitFetch`, `GitEmpty`, `GitNone`)
  is handled. A branch that returns early or logs "skipping" without doing the work
  silently breaks the feature flag for that strategy.

### Error return semantics — nil beats zero-value struct
- When a network call fails or returns undecodable data, return `nil`, not a
  zero-value struct. Callers use nil checks to detect failure; a non-nil struct
  with all-zero fields (e.g. `ID: 0`) can be mistaken for success and trigger
  downstream panics in code that expects non-nil only on success.
- When adding a guard to a legacy fallback path (e.g. content-type checks before
  re-issuing a request), ensure the guarded-off path returns `nil`, not the fallback
  result.

### Runtime identity over compile-time config
- When guarding privileged shell operations (e.g. `chown`), prefer a runtime check
  (`[ "$(id -u)" = "0" ]`) over a compile-time check of Kubernetes security context
  fields. Security context fields may not reflect reality: pods can run as non-root
  via Docker `--user`, admission webhooks, or other mechanisms not captured in the
  runner config. The runtime check is always ground truth.

### Git config file ownership
- `git config --global` resolves to `$HOME/.gitconfig`. When jobs run as non-root
  users, `$HOME` is often `/root` (owned by root), causing "Permission denied".
  Before any `git config --global` call, export `GIT_CONFIG_GLOBAL` pointing to a
  writable temp file under the runner's temp directory. Clean it up in the job
  cleanup script alongside other temp files.

### Variable expansion in secrets and external paths
- CI/CD variable references (`$VAR_NAME`) in fields like Vault secret paths, clone
  paths, and external URLs must be expanded before the value is used. Search the
  call site for an existing `ExpandVariables` / `Expand` utility — do not inline
  string replacement. If the expansion happens inside a shared interface method,
  move it there so all callers benefit and the contract is enforced centrally.
- When a path read from an env file is used as a working directory or as the
  argument to `os.RemoveAll`, validate it with `filepath.Rel(rootDir, path)` and
  reject paths that escape the root (i.e. `strings.HasPrefix(rel, "..")`). This
  guards against path-traversal via a malicious or misconfigured pre-clone script.

### Scope of a typical fix
Most merged bug fixes change 1–3 files and under 30 lines net. If your proposed
fix is larger than this, re-examine whether you are solving the right problem or
inadvertently refactoring. Flag it in the log and stop if scope has grown beyond
a targeted fix.

## Self-improvement — updating this file

After completing a fix, before pushing your branch, do the following:

1. **Identify the pattern class.** Does your fix fit an existing section in
   "Patterns from past fixes"? If yes, add a bullet to that section. If no,
   create a new `### Pattern name` subsection.

2. **Write the rule, not the story.** Record the actionable rule a future agent
   needs to apply the same fix correctly, not a narrative of what you did.
   Format: one or two sentences stating what to do (or not do), followed by
   *why* in one clause. End with `(MR !N)` referencing your MR.

3. **Update this file in the same branch as your fix.** Commit it with:
   `docs(agents): add pattern from fix for #ISSUE_IID`

4. **Keep entries concise.** If an existing bullet already covers the pattern,
   add a parenthetical `(also MR !N)` reference instead of duplicating it.

This loop ensures that each fix makes future fixes cheaper. Agents reading this
file in future conversations will inherit the accumulated knowledge of all prior
fixes without needing to rediscover the same root causes.
