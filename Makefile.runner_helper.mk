# -------------------------------------------------------------------------------
# The following make file does two things:
#   1. Create binaries for the gitlab-runner-helper app which can be found in
#   `./apps/gitlab-runner-helper` for all the platforms we want to support.
#   2. Create Linux containers and extract their file system to be used later to
#   build/publish.
#
# If you want to add a new arch or OS you would need to add a new
# file path to the $BINARIES variables and a new GO_ARCH_{{arch}}-{{OS}}
# variable. Note that Linux is implied by default.
# ---------------------------------------------------------------------------


# Tar files that we want to generate from the Docker file system, this is
# genarlly used for linux based Dockerfiles.
BASE_TAR_PATH := out/helper-images/prebuilt
TAR += ${BASE_TAR_PATH}-x86_64.tar.xz
TAR += ${BASE_TAR_PATH}-arm.tar.xz
TAR += ${BASE_TAR_PATH}-arm64.tar.xz

# Binaries that we support for the helper image. We are using the following
# pattern match:
# dockerfiles/build/binaries/gitlab-runner-helper.{{arch}}-{{os}}, these should
# match up with GO_ARCH_* variables names. Note that Linux is implied by
# default.
BASE_BINARY_PATH := dockerfiles/build/binaries/gitlab-runner-helper
BINARIES := ${BASE_BINARY_PATH}.x86_64-windows
BINARIES += ${BASE_BINARY_PATH}.x86_64
BINARIES += ${BASE_BINARY_PATH}.arm
BINARIES += ${BASE_BINARY_PATH}.arm64

# Define variables with the archiecture for each matching binary. We are using
# the following pattern match GO_ARCH_{{arch}}-{{os}}, these should match up
# with BINARIES variables. The value of the varible is the dist name from `go tool dist list`
GO_ARCH_x86_64 = linux/amd64
GO_ARCH_arm = linux/arm
GO_ARCH_arm64 = linux/arm64
GO_ARCH_x86_64-windows = windows/amd64

# Go files that are used to create the helper binary.
HELPER_GO_FILES ?= $(shell find common network vendor -name '*.go')

.PHONY: helper-build helper-docker

# PHONY command to help build the binaries.
helper-build: $(BINARIES)

dockerfiles/build/binaries/gitlab-runner-helper.%: $(HELPER_GO_FILES) $(GOX)
	gox -osarch=$(GO_ARCH_$*) -ldflags "$(GO_LDFLAGS)" -output=$@ $(PKG)/apps/gitlab-runner-helper

# PHONY command to help build the tar files for linux.
helper-docker: $(TAR)

out/helper-images/prebuilt-%.tar.xz: out/helper-images/prebuilt-%.tar
	xz -f -9 $<

out/helper-images/prebuilt-%.tar: dockerfiles/build/binaries/gitlab-runner-helper.%
	@mkdir -p $$(dirname $@_)
	docker build -t gitlab/gitlab-runner-helper:$*-$(REVISION) -f dockerfiles/build/Dockerfile.$* dockerfiles/build
	-docker rm -f gitlab-runner-prebuilt-$*-$(REVISION)
	docker create --name=gitlab-runner-prebuilt-$*-$(REVISION) gitlab/gitlab-runner-helper:$*-$(REVISION) /bin/sh
	docker export -o $@ gitlab-runner-prebuilt-$*-$(REVISION)
	docker rm -f gitlab-runner-prebuilt-$*-$(REVISION)
