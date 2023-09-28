# Writing magefiles

## Introduction

Magefiles are written in Go, and are compiled to a binary that is executed by the `mage` command. The `mage` command is a drop-in replacement for `make`, and is used in the same way.
All `mage` targets are written as functions in files contained in the `magefiles` directory. Top level files, such as `package.go` contain
simple functions that call into subpackages that contain the complex logic.

## Creating a new `mage` target

Let's create a `mage` target that cleans up the `.tmp` directory. First, we need to create a new file in the `magefiles` directory.
We'll call it `clean.go`. The file should contain the following:

```go
//go:build mage

package main

import (
    "os"

    "github.com/magefile/mage/mg"
)

type Clean mg.Namespace

// Tmp cleans the .tmp directory
func (Clean) Tmp() error {
    return os.RemoveAll(".tmp")
}
```

Running `mage` will list the target under the `clean` namespace:

```bash
$ mage
Targets:
  clean:tmp                    cleans the .tmp directory
```

All top level mage Go files should contain the `go:build mage` directive, while subpackages should not.

Subpackages are created in subdirectories and imported as normal.

## Creating complex targets with dependencies and artifacts

Complex targets that require a lot of files and environment variables are ultimately hard to figure out.
A target could fail quite a few times during its course while trying to run it locally because it requires different dependencies
at different points of its execution.

In the same way it's not easy to know what a mage target could produce and one would often rely on output to figure that out.

And lastly, without an easy way to track the artifacts of a target it could be hard to collect them and verify that they are correctly built and published.

For that, the `blueprint` could be used. The blueprint is intended to define every *dependency*, *artifact* and *environment variable* that a target requires.

-----

Let's write a mage target that builds a Docker image from a Dockerfile and pushes it as two separate tags.

Create a `test.Dockerfile` file with the following content in the root of the repo:

```dockerfile
FROM alpine

RUN apk add --no-cache curl
```

Our target will have one dependency: `test.Dockerfile`.

It will also produce two artifacts: `test:latest` and `test:1.0.0`.

It will require the following environment variables: `CI_REGISTRY`, `CI_REGISTRY_USERNAME`, `CI_REGISTRY_PASSWORD`, and `IMAGE_VERSION`.

Create a `build.go` file in the `magefiles` directory with the following content:

```go
package main

import (
	"fmt"

	"github.com/magefile/mage/sh"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/ci"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
)

var (
	// Env variables should have a default when possible to make it easy to run locally
	// Env variables are only evaluated through the blueprint
	imageVersion = env.NewDefault("IMAGE_VERSION", "v1.0.0")
)

func Build() error {
	// Print the assembled build blueprint
	blueprint := build.PrintBlueprint(assembleBuild())

	// Access environment only through the blueprint
	// this ensures they'll have correct default values and fallback keys as well as
	// ensure predictable behavior when running locally and in CI
	env := blueprint.Env()

	if err := sh.RunV("docker", "login", "-u", env.Value(ci.RegistryUser), "-p", env.Value(ci.RegistryPassword), env.Value(ci.Registry)); err != nil {
		return err
	}

	dockerfilePath := blueprint.Dependencies()[0].Value()

	if err := sh.RunV("docker", "build", "-t", "test", "-f", dockerfilePath, "."); err != nil {
		return err
	}

	for _, img := range blueprint.Artifacts() {
		if err := sh.RunV("docker", "tag", "test", img.Value()); err != nil {
			return err
		}

		if err := sh.RunV("docker", "push", img.Value()); err != nil {
			return err
		}
	}

	return nil
}

type buildBlueprint struct {
	build.BlueprintBase

	dependencies []string
	artifacts    []string
}

func (b buildBlueprint) Dependencies() []build.Component {
	// The files will be checked for existence and reported in the rendered blueprint
	return lo.Map(b.dependencies, func(s string, _ int) build.Component {
		return build.NewFile(s)
	})
}

func (b buildBlueprint) Artifacts() []build.Component {
	// Docker images will also be checked. This will show whether an image existed prior to a target start
	return lo.Map(b.artifacts, func(s string, _ int) build.Component {
		return build.NewDockerImage(s)
	})
}

func (b buildBlueprint) Data() any {
	return nil
}

func assembleBuild() build.TargetBlueprint[build.Component, build.Component, any] {
    // Define all the dependencies, artifacts and environment variables required by a target
	base := build.NewBlueprintBase(
		ci.Registry,
		ci.RegistryUser,
		ci.RegistryPassword,
		imageVersion,
	)

	dependencies := []string{"test.Dockerfile"}

	registry := base.Env().Value(ci.Registry)
	imageVersion := base.Env().Value(imageVersion)
	artifacts := []string{
		fmt.Sprintf("%s/%s:%s", registry, "test", imageVersion),
		fmt.Sprintf("%s/%s:latest", registry, "test"),
	}

	return buildBlueprint{
		BlueprintBase: base,
		dependencies:  dependencies,
		artifacts:     artifacts,
	}
}
```

Running `mage build` will prior to starting the build print the blueprint:

```bash
+---------------------------------+--------------+--------------------------------------------+
| TARGET INFO                     |              |                                            |
+---------------------------------+--------------+--------------------------------------------+
| Dependency                      | Type         | Exists                                     |
+---------------------------------+--------------+--------------------------------------------+
| test.Dockerfile                 | File         | Yes                                        |
+---------------------------------+--------------+--------------------------------------------+
| Artifact                        | Type         | Exists                                     |
+---------------------------------+--------------+--------------------------------------------+
| registry.gitlab.com/test:latest | Docker image | requested access to the resource is denied |
| registry.gitlab.com/test:v1.0.0 | Docker image | requested access to the resource is denied |
+---------------------------------+--------------+--------------------------------------------+
| Environment variable            | Is set       | Is default                                 |
+---------------------------------+--------------+--------------------------------------------+
| CI_REGISTRY                     | Yes          | Yes                                        |
| CI_REGISTRY_PASSWORD            | No           | Yes                                        |
| CI_REGISTRY_USER                | No           | Yes                                        |
| IMAGE_VERSION                   | Yes          | Yes                                        |
+---------------------------------+--------------+--------------------------------------------+
```

## Checking artifacts after a build

The blueprint allows for the artifacts to be exported to a JSON file, assembled later and checked for existence.

Add this code after a blueprint has been assembled:

```go
if err := build.Export(blueprint.Artifacts(), build.ReleaseArtifactsPath("runner_images")); err != nil {
    return err
}
```

This will create the file `out/release_artifacts/runner_images.json`.
Use the `mage resources:verify` and `resources:verifyAll` targets to verify the exported resources.
