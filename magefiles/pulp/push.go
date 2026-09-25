package pulp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jpillora/backoff"
	"github.com/magefile/mage/sh"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
)

type PushOpts struct {
	Branch      string   // Branch is the release branch ("stable" or "unstable").
	PkgType     string   // PkgType is the package type ("deb" or "rpm").
	Distro      string   // Distro is the distribution/release filter prefix (e.g., "ubuntu/focal", "fedora/43").
	Archs       []string // Archs is the list of architectures. Only relevant for RPM packages.
	Concurrency int      // Concurrency is the maximum number of concurrent uploads.
	DryRun      bool     // DryRun enables dry-run mode (no actual commands executed).
}

func Push(opts PushOpts) error {
	if err := validateInputs(opts.PkgType, opts.Branch); err != nil {
		return err
	}

	// get the distro/releases for this package-type and branch
	releases, err := releases(opts.PkgType, opts.Branch)
	if err != nil {
		return err
	}

	// filter releases by distro...
	releases = lo.Filter(releases, func(release string, _ int) bool {
		keep := strings.HasPrefix(release, opts.Distro)
		if !keep {
			slog.Debug("Skipping...", "distro", release)
		}
		return keep
	})

	if len(releases) == 0 {
		slog.Info("No releases to push for package type", "package-type", opts.PkgType)
		return nil
	}

	// get the packages to upload...
	packages, err := filepath.Glob(fmt.Sprintf("out/%s/*.%s", opts.PkgType, opts.PkgType))
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		slog.Info("No packages to push")
		return nil
	}

	// the actual repo name for the stable branch is gitlab-runner
	if opts.Branch == "stable" {
		opts.Branch = "gitlab-runner"
	}

	var p pusher
	base := basePusher{dryrun: opts.DryRun, exec: sh.Exec, branch: opts.Branch, concurrency: opts.Concurrency}
	switch opts.PkgType {
	case deb:
		p = &debPusher{basePusher: base}
	case rpm:
		p = &rpmPusher{basePusher: base, archs: opts.Archs}
	}
	return p.Push(releases, packages)
}

type (
	shExec = func(map[string]string, io.Writer, io.Writer, string, ...string) (bool, error)

	pusher interface {
		Push([]string, []string) error
	}

	basePusher struct {
		dryrun      bool
		branch      string
		concurrency int

		// testing hook
		exec shExec
	}

	debPusher struct {
		basePusher
	}
	rpmPusher struct {
		basePusher
		archs []string
	}
)

// pulpRetryErrors matches stderr output of pulp commands that failed for a reason worth retrying.
var pulpRetryErrors = []*regexp.Regexp{
	// Two concurrent uploads of the same file race to create the artifact. The loser fails with this
	// and succeeds on retry, because by then the artifact exists and is reused.
	regexp.MustCompile(`Artifact with sha256 checksum of '.*' already exists`),
	// pulp.gitlab.com sits behind Cloudflare, which renders 5xx responses from the origin as an HTML
	// error page carrying the status code.
	regexp.MustCompile(`Error reference number: 5\d\d`),
	regexp.MustCompile(`(?i)bad gateway|gateway time-?out|service unavailable|internal server error`),
	// A Pulp task fails like this when the worker running it dies, for example when it runs out of memory.
	regexp.MustCompile(`Worker has gone missing`),
	// Connection-level failures between the pulp CLI and the API.
	regexp.MustCompile(`(?i)connection (reset|aborted|refused)|remote end closed connection|read timed out|max retries exceeded`),
}

// retryPulpCmd runs a pulp command, retrying it on transient failures. Once the command succeeds its
// stdout is written to out. Waiting for a retry is abandoned when ctx is cancelled, which happens
// when another upload of the same push has already failed for good.
func (p *basePusher) retryPulpCmd(ctx context.Context, args []string, out io.Writer) error {
	slog.Info("executing", "cmd", "pulp", "args", args)
	if p.dryrun {
		return nil
	}

	return newRetryCommand("pulp", args, pulpRetryErrors, out, p.exec).run(ctx)
}

// newPool returns a pool that stops starting uploads as soon as one of them has failed for good.
// All uploads of a push go to the same Pulp instance, so once one has exhausted its retries the
// others will not fare any better, and failing right away turns hours of doomed uploads into
// minutes. Uploads already in flight run to completion, since the pulp CLI is not interruptible.
func (p *basePusher) newPool(ctx context.Context) *pool.ContextPool {
	return pool.New().
		WithMaxGoroutines(p.concurrency).
		WithErrors().
		WithContext(ctx).
		WithCancelOnError()
}

// runUnlessAborted skips fn, described by what, when the push has already been aborted by a
// failed upload.
func runUnlessAborted(ctx context.Context, what string, fn func() error) error {
	if ctx.Err() != nil {
		slog.Warn("skipping, an earlier upload failed for good", "skipped", what)
		return nil
	}
	return fn()
}

func (p *basePusher) execCmd(out io.Writer, cmd string, args ...string) error {
	slog.Info("executing", "cmd", cmd, "args", args)
	if p.dryrun {
		return nil
	}
	_, err := p.exec(nil, out, os.Stderr, cmd, args...)
	return err
}

// For deb packages, the pulp repo is configured such that:
//   - The arch will be auto-detected, so does not need to be specified.
//   - There's a single repo per distribution, handling all releases for that distribution.
//   - Every package must be uploaded once per distro/release/arch. Each upload creates a new
//     repository version, which by itself takes seconds.
//   - Publishing takes minutes and grows with the repo, so it is done once per repository after
//     all uploads, unless the repository still autopublishes every version.
//   - There's no special handling of the gitlab-runner-helper-images package; its arch is "all".
func (p *debPusher) Push(releases, pkgFiles []string) error {
	slog.Debug("Will push the following packages to pulp", "packages", pkgFiles, "releases", releases)
	ctx := context.Background()
	pool := p.newPool(ctx)
	for _, release := range releases {
		for _, pkgFile := range pkgFiles {
			pool.Go(func(ctx context.Context) error {
				return runUnlessAborted(ctx, "upload of "+pkgFile+" to "+release, func() error {
					return p.retryPulpCmd(ctx, p.pushArgs(release, pkgFile), io.Discard)
				})
			})
		}
	}
	if err := pool.Wait(); err != nil {
		return err
	}

	for _, repo := range p.pulpRepos(releases) {
		if err := p.publish(ctx, repo); err != nil {
			return fmt.Errorf("publishing repo %s: %w", repo, err)
		}
	}
	return nil
}

func (p *debPusher) pulpRepo(release string) string {
	return "runner-" + p.branch + "-" + strings.Split(release, "/")[0]
}

// pulpRepos returns the distinct repositories the releases live in, in a stable order.
func (p *debPusher) pulpRepos(releases []string) []string {
	repos := lo.Uniq(lo.Map(releases, func(release string, _ int) string { return p.pulpRepo(release) }))
	slices.Sort(repos)
	return repos
}

func (p *debPusher) pushArgs(release, pkg string) []string {
	return []string{
		deb, "content", "upload", "--file", pkg,
		"--distribution", strings.Split(release, "/")[1],
		"--component", "main",
		"--repository", p.pulpRepo(release),
		"--chunk-size", "10MB",
	}
}

// publish creates one publication of the repository's latest version, which makes everything
// uploaded by this push visible at once. A repository that still autopublishes every new version
// is left alone: it has published already, and the CI user may not hold the publication
// permission yet. Once autopublish is turned off on the Pulp side this takes over without any
// further change here.
func (p *debPusher) publish(ctx context.Context, repo string) error {
	autopublish, err := p.autopublishes(ctx, repo)
	if err != nil {
		return err
	}
	if autopublish {
		slog.Info("repository autopublishes every version, not creating a publication", "repository", repo)
		return nil
	}
	return p.retryPulpCmd(ctx, []string{deb, "publication", "create", "--repository", repo}, io.Discard)
}

func (p *debPusher) autopublishes(ctx context.Context, repo string) (bool, error) {
	if p.dryrun {
		return false, nil
	}

	out := bytes.Buffer{}
	if err := p.retryPulpCmd(ctx, []string{deb, "repository", "show", "--name", repo}, &out); err != nil {
		return false, err
	}

	var result struct {
		Autopublish bool `json:"autopublish"`
	}
	if err := json.NewDecoder(&out).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to unmarshal repository %s: %w", repo, err)
	}
	return result.Autopublish, nil
}

const (
	helperImagePkg = "gitlab-runner-helper-images"
)

// For rpm packages, the pulp repo is configured such that:
// * The arch will NOT be auto-detected; it is encoded in the pulp repo name.
// * There's one repo per distribution/release/arch tuple, e.g. fedora-43-x86_64
// * Multiple repos can point to and expose the same package/file. This means...
// * We can upload a package/file once, (to any repo), then link it to all the other relevant repos.
// * All packages/files are handled this way, including the gitlab-runner-helper-images package.
// * The only difference is that this same file is linked to repos for all archs.
func (p *rpmPusher) Push(releases, pkgFiles []string) error {
	slog.Debug("Will push the following packages to pulp", "packages", pkgFiles, "releases", releases)
	pool := p.newPool(context.Background())
	for _, pkgFile := range pkgFiles {
		pool.Go(func(ctx context.Context) error {
			return runUnlessAborted(ctx, "push of "+pkgFile, func() error {
				pkgInfo, err := p.getRPMInfo(pkgFile)
				if err != nil {
					return fmt.Errorf("failed to get rpm package info from file %s: %w", pkgFile, err)
				}
				// for runner packages we only specify one arch, corresponding to the package's arch.
				archs := []string{pkgInfo.arch}
				if pkgInfo.name == helperImagePkg {
					// for the helper images package we specify all the archs.
					archs = p.archs
				}
				return p.pushPackage(ctx, pkgFile, pkgInfo, releases, archs)
			})
		})
	}

	return pool.Wait()
}

// Even though `i686` is the correct arch label for rpm packages, pulp wants `i386`.
// See https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6038#note_2992748581
func (p *rpmPusher) normalizeArch(arch string) string {
	if arch == "i686" {
		return "i386"
	}
	return arch
}

func (p *rpmPusher) pulpRepo(release, arch string) string {
	return "runner-" + p.branch + "-" + strings.ReplaceAll(release, "/", "-") + "-" + arch
}

func (p *rpmPusher) pushArgs(pkgFile, repo string) []string {
	return []string{rpm, "content", "upload", "--file", pkgFile, "--repository", repo, "--chunk-size", "10MB"}
}

func (p *rpmPusher) linkArgs(repo, href string) []string {
	return []string{rpm, "repository", "content", "modify", "--repository", repo, "--add-content", `[{"pulp_href": "` + href + `"}]`}
}

// Push the specific package file to all the specified releases and architectures
func (p *rpmPusher) pushPackage(ctx context.Context, pkgFile string, pkgInfo rpmInfo, releases, archs []string) error {
	archs = lo.Map(archs, func(a string, _ int) string {
		return p.normalizeArch(a)
	})

	// push the package to the fist release/arch
	repo := p.pulpRepo(releases[0], archs[0])
	slog.Debug("Pushing", "package", pkgFile, "release", releases[0], "arch", archs[0], "version", pkgInfo.version)
	href, err := p.doPush(ctx, pkgFile, repo)
	if err != nil {
		return fmt.Errorf("failed to push file %s to repo %s: %w", pkgFile, repo, err)
	}

	slog.Debug("Package successfully uploaded", "file", pkgFile, "pulp_href", href)

	// link the package to all other relevant releases/archs
	for _, release := range releases {
		for _, arch := range archs {
			repo := p.pulpRepo(release, arch)
			err = errors.Join(err, runUnlessAborted(ctx, "link of "+pkgFile+" to "+repo, func() error {
				slog.Debug("Linking", "package", pkgFile, "release", release, "arch", arch, "version", pkgInfo.version)
				return p.retryPulpCmd(ctx, p.linkArgs(repo, href), io.Discard)
			}))
		}
	}
	return err
}

type repoPushResult struct {
	PulpHref string `json:"pulp_href,omitempty"`
}

func (p *rpmPusher) doPush(ctx context.Context, pkgFile, repo string) (string, error) {
	args := p.pushArgs(pkgFile, repo)

	out := bytes.Buffer{}
	if err := p.retryPulpCmd(ctx, args, &out); err != nil {
		return "", err
	}

	result := repoPushResult{}
	err := json.NewDecoder(&out).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.PulpHref == "" {
		return "", fmt.Errorf("package upload response had empty 'pulp_href'")
	}
	return result.PulpHref, nil
}

// getRPMInfo runs rpm -qi on the given filename and extracts the Version, Name and Architecture fields
func (p *rpmPusher) getRPMInfo(filename string) (rpmInfo, error) {
	out := bytes.Buffer{}
	if err := p.execCmd(&out, rpm, "-qi", filename); err != nil {
		return rpmInfo{}, fmt.Errorf("failed to query rpm package %q: %w", filename, err)
	}

	// Parse the output to extract Version field
	info, err := parseRPMInfo(&out)
	if err != nil {
		return rpmInfo{}, fmt.Errorf("failed to parse version from rpm output: %w", err)
	}

	return info, nil
}

var (
	versionRE = regexp.MustCompile(`Version\s*:\s*([^ ]+)\s*`)
	archRE    = regexp.MustCompile(`Architecture\s*:\s*([^ ]+)\s*`)
	nameRE    = regexp.MustCompile(`Name\s*:\s*([^ ]+)\s*`)
)

type rpmInfo struct {
	name    string
	version string
	arch    string
}

func (i *rpmInfo) parseLine(line string) bool {
	if matches := versionRE.FindStringSubmatch(line); len(matches) == 2 {
		i.version = matches[1]
	} else if matches := nameRE.FindStringSubmatch(line); len(matches) == 2 {
		i.name = matches[1]
	} else if matches := archRE.FindStringSubmatch(line); len(matches) == 2 {
		i.arch = matches[1]
	}

	return i.allFieldsFound()
}

func (i *rpmInfo) allFieldsFound() bool {
	return i.name != "" && i.arch != "" && i.version != ""
}

// parseRPMInfo extracts the Version field from rpm -qi output.
// The output is not structured, but contains lines like:
// "Version      : <version>"
// "Architecture : <arch>"
// "Name         : <name>"
func parseRPMInfo(out io.Reader) (rpmInfo, error) {
	scanner := bufio.NewScanner(out)
	info := rpmInfo{}

	for scanner.Scan() {
		if info.parseLine(scanner.Text()) {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return rpmInfo{}, fmt.Errorf("error reading rpm output: %w", err)
	}

	if !info.allFieldsFound() {
		return rpmInfo{}, fmt.Errorf("at least one field not found in rpm output")
	}

	return info, nil
}

type retryCommand struct {
	cmd           string
	args          []string
	backoff       backoff.Backoff
	out           io.Writer
	retryableErrs []*regexp.Regexp
	exec          shExec
}

func newRetryCommand(cmd string, args []string, retryableErrs []*regexp.Regexp, out io.Writer, exec shExec) *retryCommand {
	return &retryCommand{
		cmd:  cmd,
		args: args,
		backoff: backoff.Backoff{
			Min:    15 * time.Second,
			Max:    2 * time.Minute,
			Factor: 2,
			Jitter: true,
		},
		out:           out,
		retryableErrs: retryableErrs,
		exec:          exec,
	}
}

const maxAttempts = 5

func (c *retryCommand) run(ctx context.Context) error {
	var lastStderr string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			delay := c.backoff.Duration()
			slog.Warn("command failed with a retryable error, waiting before retrying", "delay", delay, "attempt", attempt-1, "command", c.cmd, "args", c.args)
			select {
			case <-ctx.Done():
				return fmt.Errorf("execution of command (%s %s) abandoned after attempt %d: %w", c.cmd, c.args, attempt-1, ctx.Err())
			case <-time.After(delay):
			}
		}

		slog.Info("attempting to run command", "attempt", attempt, "command", c.cmd, "args", c.args)

		outBuf, errBuf := bytes.Buffer{}, bytes.Buffer{}
		stdout := io.MultiWriter(&outBuf, os.Stdout)
		stderr := io.MultiWriter(&errBuf, os.Stderr)

		_, err := c.exec(nil, stdout, stderr, c.cmd, c.args...)

		if err == nil {
			_, _ = io.Copy(c.out, &outBuf)
			return nil
		}
		lastStderr = errBuf.String()
		if !c.isRetryable(lastStderr) {
			return fmt.Errorf("execution of command (%s %s) failed: %s", c.cmd, c.args, lastStderr)
		}
	}

	return fmt.Errorf("execution of command (%s %s) failed after %d attempts: %s", c.cmd, c.args, maxAttempts, lastStderr)
}

func (c *retryCommand) isRetryable(stderr string) bool {
	for _, re := range c.retryableErrs {
		if re.MatchString(stderr) {
			return true
		}
	}
	return false
}
