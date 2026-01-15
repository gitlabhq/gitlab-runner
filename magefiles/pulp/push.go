import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sourcegraph/conc/pool"
)

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
)

func (p *basePusher) runPulpCmd(args ...string) error {
	fmt.Println("executing", "pulp", strings.Join(args, " "))
	if p.dryrun {
		return nil
	}
	return p.run("pulp", args...)
}

func (p *basePusher) execCmd(out io.Writer, cmd string, args ...string) error {
	fmt.Println("executing", cmd, strings.Join(args, " "))
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
				slog.Debug("Pushing", "package", pkgFile, "release", release)
				return p.runPulpCmd(p.pushArgs(release, pkgFile)...)
			})
		}
	}

	return pool.Wait()
}

func (p *debPusher) pushArgs(release, pkg string) []string {
	pulpRepo := "runner-" + p.branch + "-" + strings.Split(release, "/")[0]
	return []string{
		"deb", "content", "upload", "--file", pkg,
		"--distribution", strings.Split(release, "/")[1],
		"--component", "main",
		"--repository", pulpRepo,
	}
}
