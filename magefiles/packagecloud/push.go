package packagecloud

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
	"github.com/sourcegraph/conc/pool"
)

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

					return sh.RunV(
						"package_cloud",
						args...,
					)
				})
			}
		}
	}

	return pool.Wait()
}
