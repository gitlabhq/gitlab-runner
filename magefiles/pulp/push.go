package pulp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
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
	Archs       []string // Archs is the list of architectures. Only relevant for for RPM packages.
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
	base := basePusher{dryrun: opts.DryRun, run: sh.Run, exec: sh.Exec, branch: opts.Branch, concurrency: opts.Concurrency}
	switch opts.PkgType {
	case deb:
		p = &debPusher{basePusher: base}
	case rpm:
		p = &rpmPusher{basePusher: base, archs: opts.Archs}
	}
	return p.Push(releases, packages)
}

type (
	shRun  = func(string, ...string) error
	shExec = func(map[string]string, io.Writer, io.Writer, string, ...string) (bool, error)

	pusher interface {
		Push([]string, []string) error
	}

	basePusher struct {
		dryrun      bool
		branch      string
		concurrency int

		// testing hooks
		exec shExec
		run  shRun
	}

	debPusher struct {
		basePusher
	}
	rpmPusher struct {
		basePusher
		archs []string
	}
)

func (p *basePusher) runPulpCmd(args ...string) error {
	slog.Info("executing", "cmd", "pulp", "args", args)
	if p.dryrun {
		return nil
	}
	return p.run("pulp", args...)
}

var pulpRetryErrors = []*regexp.Regexp{
	regexp.MustCompile(`Artifact with sha256 checksum of '.*' already exists`),
}

func (p *basePusher) retryPulpCmd(args []string, out io.Writer) error {
	slog.Info("executing", "cmd", "pulp", "args", args)
	if p.dryrun {
		return nil
	}

	return newRetryCommand("pulp", args, pulpRetryErrors, out, p.exec).run()
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
// * The arch will be auto-detected, so does not need to be specified.
// * There's a single repo per distribution, handling all releases for that distribution.
// * Every package must be uploaded once per distro/release/arch.
// * There's no special handling of the gitlab-runner-helper-images package; its arch is "all".
func (p *debPusher) Push(releases, pkgFiles []string) error {
	slog.Debug("Will push the following packages to pulp", "packages", pkgFiles, "releases", releases)
	pool := pool.New().WithMaxGoroutines(p.concurrency).WithErrors()
	for _, release := range releases {
		for _, pkgFile := range pkgFiles {
			pool.Go(func() error {
				return p.retryPulpCmd(p.pushArgs(release, pkgFile), io.Discard)
			})
		}
	}

	return pool.Wait()
}

func (p *debPusher) pushArgs(release, pkg string) []string {
	pulpRepo := "runner-" + p.branch + "-" + strings.Split(release, "/")[0]
	return []string{
		deb, "content", "upload", "--file", pkg,
		"--distribution", strings.Split(release, "/")[1],
		"--component", "main",
		"--repository", pulpRepo,
		"--chunk-size", "10MB",
	}
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
	pool := pool.New().WithMaxGoroutines(p.concurrency).WithErrors()
	for _, pkgFile := range pkgFiles {
		pool.Go(func() error {
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
			return p.pushPackage(pkgFile, pkgInfo, releases, archs)
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
func (p *rpmPusher) pushPackage(pkgFile string, pkgInfo rpmInfo, releases, archs []string) error {
	archs = lo.Map(archs, func(a string, _ int) string {
		return p.normalizeArch(a)
	})

	// push the package to the fist release/arch
	repo := p.pulpRepo(releases[0], archs[0])
	slog.Debug("Pushing", "package", pkgFile, "release", releases[0], "arch", archs[0], "version", pkgInfo.version)
	href, err := p.doPush(pkgFile, repo)
	if err != nil {
		return fmt.Errorf("failed to push file %s to repo %s: %w", pkgFile, repo, err)
	}

	slog.Debug("Package successfully uploaded", "file", pkgFile, "pulp_href", href)

	// link the package to all other relevant releases/archs
	for _, release := range releases {
		for _, arch := range archs {
			slog.Debug("Linking", "package", pkgFile, "release", release, "arch", arch, "version", pkgInfo.version)
			repo := p.pulpRepo(release, arch)
			err = errors.Join(err, p.runPulpCmd(p.linkArgs(repo, href)...))
		}
	}
	return err
}

type repoPushResult struct {
	PulpHref string `json:"pulp_href,omitempty"`
}

func (p *rpmPusher) doPush(pkgFile, repo string) (string, error) {
	args := p.pushArgs(pkgFile, repo)

	out := bytes.Buffer{}
	if err := p.retryPulpCmd(args, &out); err != nil {
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
			Min: time.Second,
			Max: 5 * time.Second,
		},
		out:           out,
		retryableErrs: retryableErrs,
		exec:          exec,
	}
}

func (c *retryCommand) run() error {
	for i := range 5 {
		slog.Info("attempting to run command", "attempt", i+1, "command", c.cmd, "args", c.args)

		outBuf, errBuf := bytes.Buffer{}, bytes.Buffer{}
		stdout := io.MultiWriter(&outBuf, os.Stdout)
		stderr := io.MultiWriter(&errBuf, os.Stderr)

		_, err := c.exec(nil, stdout, stderr, c.cmd, c.args...)

		if err == nil {
			_, _ = io.Copy(c.out, &outBuf)
			return nil
		}
		if c.isRetryable(errBuf.String()) {
			time.Sleep(c.backoff.Duration())
			continue
		}
		return fmt.Errorf("execution of command (%s %s) failed: %s", c.cmd, c.args, errBuf.String())
	}

	return fmt.Errorf("execution of command (%s %s) failed after 5 retries ", c.cmd, c.args)
}

func (c *retryCommand) isRetryable(stderr string) bool {
	for _, re := range c.retryableErrs {
		if re.MatchString(stderr) {
			return true
		}
	}
	return false
}
