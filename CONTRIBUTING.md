## Contributing to GitLab Runner

Contributions are welcome!

The official repository for this project is on [GitLab.com](https://gitlab.com/gitlab-org/gitlab-ci-multi-runner).

* [Development](docs/development/README.md)
* [Issues](https://gitlab.com/gitlab-org/gitlab-ci-multi-runner/issues)
* [Merge Requests](https://gitlab.com/gitlab-org/gitlab-ci-multi-runner/merge_requests)

## Contributor license agreement

By submitting code as an individual you agree to the
[individual contributor license agreement](https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/legal/individual_contributor_license_agreement.md).
By submitting code as an entity you agree to the
[corporate contributor license agreement](https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/legal/corporate_contributor_license_agreement.md).

## Managing build dependencies

GitLab Runner uses [Godep](https://github.com/tools/godep) to manage its dependencies - they get checked into the repository under the `vendor/` directory, with a manifest
stored in `Godeps/Godeps.json`.

If your contribution adds, removes or updates any dependencies to the runner,
please ensure the vendored copies updated in lock-step. If you're new to Go, or
Godep, here is a conservative process to follow:

* Start with a completely empty GOPATH with the right Go version (currently 1.7.1). [GVM](https://github.com/moovweb/gvm) makes this easy.
* Clone the repository into this GOPATH, e.g.: `go get gitlab.com/gitlab-org/gitlab-ci-multi-runner`
* Run `go get github.com/tools/godep`
* Change directory: `cd $GOPATH/src/gitlab.com/gitlab-org/gitlab-ci-multi-runner`
* Run `godep restore -v && rm -rf Godeps vendor` to un-vendor the dependencies.
* Make your changes, running `go get [-u] <packages>` as necessary to add or update packages.
* When finished, run `godep save -v ./...` to recreate the Godeps and vendor directories
* Check that all the added or changed dependencies are compatible with the runner's [license](LICENSE).
* Commit your changes in one commit (or set of commits), followed by the changes to `Godeps` and `vendor/` in a final commit, then push to a fork or branch.
* Check out your branch in a new clean GOPATH and ensure `make verify` runs without error.
* Open a merge request!
