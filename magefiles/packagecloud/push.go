package packagecloud

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jpillora/backoff"
	"github.com/magefile/mage/sh"
	"github.com/sourcegraph/conc/pool"
)

var (
	ignoredPackageCloudErrors = []string{
		"architecture: Unrecognized CPU architecture",
		"filename: has already been taken",
	}

	retryPackageCloudErrors = []string{
		"502 Bad Gateway",
	}
)

type packageCloudError struct {
	err string
}

func newPackageCloudError(err string) *packageCloudError {
	return &packageCloudError{err: err}
}

func (p *packageCloudError) isIgnored() bool {
	for _, msg := range ignoredPackageCloudErrors {
		if strings.Contains(p.err, msg) {
			return true
		}
	}

	return false
}

func (p *packageCloudError) isRetryable() bool {
	for _, msg := range retryPackageCloudErrors {
		if strings.Contains(p.err, msg) {
			return true
		}
	}

	return false
}

func (p *packageCloudError) Error() string {
	return p.err
}

type PushOpts struct {
	URL         string
	Namespace   string
	Token       string
	Branch      string
	Dist        string
	Flavor      string
	Concurrency int
	DryRun      bool
}

func Push(opts PushOpts) error {
	releases, err := Releases(opts.Dist, opts.Branch, opts.Token, opts.URL)
	if err != nil {
		return err
	}

	packages, err := filepath.Glob(fmt.Sprintf("out/%s/*.%s", opts.Dist, opts.Dist))
	if err != nil {
		return err
	}

	pool := pool.New().WithMaxGoroutines(opts.Concurrency).WithErrors()
	for _, release := range releases {
		release := release
		if opts.Flavor == "" || strings.Contains(release, opts.Flavor) {
			for _, pkg := range packages {
				pkg := pkg
				pool.Go(func() error {
					args := []string{
						"push",
						"--verbose",
						"--url",
						opts.URL,
						fmt.Sprintf("%s/%s", opts.Namespace, release),
						pkg,
					}

					fmt.Println("Pushing to PackageCloud with args: ", args)
					if opts.DryRun {
						return nil
					}

					return newPackageCloudCommand(args).run()
				})
			}
		}
	}

	return pool.Wait()
}

type packageCloudCommand struct {
	cmd     string
	args    []string
	backoff backoff.Backoff
}

func newPackageCloudCommand(args []string) *packageCloudCommand {
	return &packageCloudCommand{
		cmd:  "package_cloud",
		args: args,
		backoff: backoff.Backoff{
			Min: time.Second,
			Max: 5 * time.Second,
		},
	}
}

func (p *packageCloudCommand) run() error {
	for i := 0; i < 5; i++ {
		time.Sleep(p.backoff.Duration())
		fmt.Printf("Running PackageCloud upload command try #%d\n", i+1)

		var out bytes.Buffer

		stdout := io.MultiWriter(&out, os.Stdout)
		stderr := io.MultiWriter(&out, os.Stderr)

		_, err := sh.Exec(
			nil,
			stdout,
			stderr,
			"package_cloud",
			p.args...,
		)

		pkgCloudErr := newPackageCloudError(out.String())
		if err == nil || pkgCloudErr.isIgnored() {
			return nil
		} else if pkgCloudErr.isRetryable() {
			continue
		}

		return err
	}

	return fmt.Errorf("failed to run PackageCloud command after 5 tries")
}
