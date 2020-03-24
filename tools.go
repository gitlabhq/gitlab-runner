// +build tools

package main

// These imports are to force `go mod tidy` not to remove that tools we depend
// on development. This is explained in great detail in
// https://marcofranssen.nl/manage-go-tools-via-go-modules/
import (
	_ "github.com/mitchellh/gox"              // cross-compilation of the binary
	_ "github.com/vektra/mockery/cmd/mockery" // automatic generation of the mocks
)
