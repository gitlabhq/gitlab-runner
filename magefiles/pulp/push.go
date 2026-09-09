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
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
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

func Push(ctx context.Context, opts PushOpts) error {
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
		dp := &debPusher{basePusher: base}
		if !opts.DryRun {
			if dp.api, err = newAPIClientFromCLIConfig(); err != nil {
				return err
			}
		}
		p = dp
	case rpm:
		p = &rpmPusher{basePusher: base, archs: opts.Archs}
	}
	return p.Push(ctx, releases, packages)
}

type (
	shRun  = func(string, ...string) error
	shExec = func(map[string]string, io.Writer, io.Writer, string, ...string) (bool, error)

	pusher interface {
		Push(ctx context.Context, releases, pkgFiles []string) error
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
		api contentAPI
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

// contentAPI is the part of the Pulp REST API used for deb content units pulp-cli cannot create.
type contentAPI interface {
	createContent(ctx context.Context, path string, body any) (string, error)
	findContent(ctx context.Context, path string, query map[string]string) (string, error)
}

const (
	debComponent                = "main"
	debReleaseArchitecturesPath = "/pulp/api/v3/content/deb/release_architectures/"
	debPackageReleaseCompsPath  = "/pulp/api/v3/content/deb/package_release_components/"
)

type debPackage struct {
	PulpHref     string `json:"pulp_href"`
	Architecture string `json:"architecture"`
}

// For deb packages, the pulp repo is configured such that:
//   - There's a single repo per distribution (e.g. runner-unstable-ubuntu), handling all releases for that distribution.
//   - A package is visible in a release when the repo contains a package_release_component unit linking
//     the package to the release's component, plus a release_architecture unit for the package's arch.
//   - Every new repository version triggers a publish of the whole repo, which is the expensive part.
//
// So each file is uploaded once, the link units are created without touching the repo, and everything
// is added to the repo in a single modify call, giving one version and one publish per repo.
func (p *debPusher) Push(ctx context.Context, releases, pkgFiles []string) error {
	slog.Debug("Will push the following packages to pulp", "packages", pkgFiles, "releases", releases)

	packages, err := p.uploadPackages(pkgFiles)
	if err != nil {
		return err
	}

	byRepo := map[string][]string{}
	for _, release := range releases {
		repo := p.pulpRepo(release)
		byRepo[repo] = append(byRepo[repo], release)
	}

	for _, repo := range slices.Sorted(maps.Keys(byRepo)) {
		if err := p.pushToRepo(ctx, repo, byRepo[repo], packages); err != nil {
			return fmt.Errorf("pushing to repo %s: %w", repo, err)
		}
	}
	return nil
}

func (p *debPusher) uploadPackages(pkgFiles []string) ([]debPackage, error) {
	var mu sync.Mutex
	packages := make([]debPackage, 0, len(pkgFiles))

	pool := pool.New().WithMaxGoroutines(p.concurrency).WithErrors()
	for _, pkgFile := range pkgFiles {
		pool.Go(func() error {
			pkg, err := p.upload(pkgFile)
			if err != nil {
				return fmt.Errorf("uploading %s: %w", pkgFile, err)
			}
			mu.Lock()
			packages = append(packages, pkg)
			mu.Unlock()
			return nil
		})
	}
	if err := pool.Wait(); err != nil {
		return nil, err
	}

	slices.SortFunc(packages, func(a, b debPackage) int { return strings.Compare(a.PulpHref, b.PulpHref) })
	return packages, nil
}

func (p *debPusher) upload(pkgFile string) (debPackage, error) {
	args := []string{deb, "content", "upload", "--file", pkgFile, "--chunk-size", "10MB"}
	if p.dryrun {
		slog.Info("executing", "cmd", "pulp", "args", args)
		return debPackage{PulpHref: "dry-run:" + pkgFile, Architecture: "dry-run"}, nil
	}

	out := bytes.Buffer{}
	if err := p.retryPulpCmd(args, &out); err != nil {
		return debPackage{}, err
	}

	pkg := debPackage{}
	if err := json.NewDecoder(&out).Decode(&pkg); err != nil {
		return debPackage{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if pkg.PulpHref == "" || pkg.Architecture == "" {
		return debPackage{}, fmt.Errorf("package upload response missing pulp_href or architecture: %+v", pkg)
	}
	return pkg, nil
}

// pushToRepo links every package to every release of the repo, then adds all units in one modify.
func (p *debPusher) pushToRepo(ctx context.Context, repo string, releases []string, packages []debPackage) error {
	archs := lo.Uniq(lo.Map(packages, func(pkg debPackage, _ int) string { return pkg.Architecture }))
	slices.Sort(archs)

	hrefs := lo.Map(packages, func(pkg debPackage, _ int) string { return pkg.PulpHref })
	var mu sync.Mutex

	pool := pool.New().WithMaxGoroutines(p.concurrency).WithErrors()
	for _, release := range releases {
		codename := strings.Split(release, "/")[1]
		pool.Go(func() error {
			units, err := p.linkRelease(ctx, codename, archs, packages)
			if err != nil {
				return err
			}
			mu.Lock()
			hrefs = append(hrefs, units...)
			mu.Unlock()
			return nil
		})
	}
	if err := pool.Wait(); err != nil {
		return err
	}

	return p.modify(repo, hrefs)
}

// linkRelease creates the units that make the packages part of a release: the release component,
// one release architecture per arch, and one package/release-component link per package.
func (p *debPusher) linkRelease(ctx context.Context, codename string, archs []string, packages []debPackage) ([]string, error) {
	componentHref, err := p.createReleaseComponent(codename)
	if err != nil {
		return nil, err
	}
	units := []string{componentHref}

	for _, arch := range archs {
		href, err := p.createContent(ctx, debReleaseArchitecturesPath, map[string]string{
			"distribution": codename,
			"architecture": arch,
		})
		if err != nil {
			return nil, fmt.Errorf("creating release architecture %s/%s: %w", codename, arch, err)
		}
		units = append(units, href)
	}

	for _, pkg := range packages {
		href, err := p.findOrCreateContent(ctx, debPackageReleaseCompsPath, map[string]string{
			"package":           pkg.PulpHref,
			"release_component": componentHref,
		})
		if err != nil {
			return nil, fmt.Errorf("linking %s to %s: %w", pkg.PulpHref, codename, err)
		}
		units = append(units, href)
	}
	return units, nil
}

// Pulp creates a new package_release_component on every POST, unlike release_component and
// release_architecture which return the existing unit, so look it up first to keep retries idempotent.
func (p *debPusher) findOrCreateContent(ctx context.Context, path string, body map[string]string) (string, error) {
	if !p.dryrun {
		href, err := p.api.findContent(ctx, path, body)
		if err != nil {
			return "", err
		}
		if href != "" {
			return href, nil
		}
	}
	return p.createContent(ctx, path, body)
}

func (p *debPusher) createReleaseComponent(codename string) (string, error) {
	args := []string{deb, "content", "-t", "release_component", "create", "--distribution", codename, "--component", debComponent}
	if p.dryrun {
		slog.Info("executing", "cmd", "pulp", "args", args)
		return "dry-run:release_component/" + codename, nil
	}

	out := bytes.Buffer{}
	if err := p.execCmd(&out, "pulp", args...); err != nil {
		return "", fmt.Errorf("creating release component for %s: %w", codename, err)
	}

	unit := contentUnit{}
	if err := json.NewDecoder(&out).Decode(&unit); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if unit.PulpHref == "" {
		return "", fmt.Errorf("release component response had empty 'pulp_href'")
	}
	return unit.PulpHref, nil
}

func (p *debPusher) createContent(ctx context.Context, path string, body map[string]string) (string, error) {
	if p.dryrun {
		slog.Info("creating pulp content", "path", path, "body", body)
		return "dry-run:" + path, nil
	}
	return p.api.createContent(ctx, path, body)
}

func (p *debPusher) modify(repo string, hrefs []string) error {
	slices.Sort(hrefs)
	content, err := json.Marshal(lo.Map(hrefs, func(href string, _ int) contentUnit { return contentUnit{PulpHref: href} }))
	if err != nil {
		return err
	}

	f, err := os.CreateTemp("", "pulp-add-content-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(content); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Close(); err != nil {
		return err
	}

	slog.Info("adding content to repository", "repository", repo, "units", len(hrefs))
	return p.runPulpCmd(deb, "repository", "content", "modify", "--repository", repo, "--add-content", "@"+f.Name())
}

func (p *debPusher) pulpRepo(release string) string {
	return "runner-" + p.branch + "-" + strings.Split(release, "/")[0]
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
func (p *rpmPusher) Push(_ context.Context, releases, pkgFiles []string) error {
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
