// +build tools

package main

// These imports are to force `go mod tidy` not to remove that tools we depend
// on development. This is explained in great detail in
// https://marcofranssen.nl/manage-go-tools-via-go-modules/
import (
	_ "github.com/mitchellh/gox"
	_ "github.com/vektra/mockery/cmd/mockery"
	_ "gitlab.com/gitlab-org/ci-cd/runner-tools/release-index-generator/cmd/release-index-gen"
)
