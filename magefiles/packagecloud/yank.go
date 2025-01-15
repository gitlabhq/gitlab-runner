package packagecloud

import (
	"fmt"

	"github.com/magefile/mage/sh"
	"github.com/sourcegraph/conc/pool"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/packages"
)

type YankOpts struct {
	Version       string
	PackageBuilds packages.Builds
	Token         string
	URL           string
	Namespace     string
	Concurrency   int
	DryRun        bool
}

func Yank(opts YankOpts) error {
	pool := pool.New().WithErrors().WithMaxGoroutines(opts.Concurrency)

	for dist := range opts.PackageBuilds {
		releases, err := Releases(dist, "stable", opts.Token, opts.URL, false)
		if err != nil {
			return err
		}

		files := packages.Filenames(opts.PackageBuilds, dist, opts.Version)
		for _, release := range releases {
			for _, file := range files {
				release := release
				file := file
				pool.Go(func() error {
					args := []string{
						"yank",
						"--url",
						opts.URL,
						fmt.Sprintf("%s/%s", opts.Namespace, release),
						file,
					}

					fmt.Println("Running yank command:", args)
					if opts.DryRun {
						return nil
					}

					return sh.RunV("package_cloud", args...)
				})
			}
		}
	}

	return pool.Wait()
}
