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

TAR_XZ_ARGS ?= -f -0

# Tar files that we want to generate from the Docker file system, this is
# generally used for linux based Dockerfiles.
BASE_TAR_PATH := out/helper-images/prebuilt

TAR_XZ_ALPINE += ${BASE_TAR_PATH}-alpine-x86_64.tar.xz
TAR_XZ_ALPINE += ${BASE_TAR_PATH}-alpine-x86_64-pwsh.tar.xz
TAR_XZ_ALPINE += ${BASE_TAR_PATH}-alpine-arm.tar.xz
TAR_XZ_ALPINE += ${BASE_TAR_PATH}-alpine-arm64.tar.xz
TAR_XZ_ALPINE += ${BASE_TAR_PATH}-alpine-s390x.tar.xz
TAR_XZ_ALPINE += ${BASE_TAR_PATH}-alpine-ppc64le.tar.xz

TAR_XZ_ALPINE_313 += ${BASE_TAR_PATH}-alpine3.13-x86_64.tar.xz
TAR_XZ_ALPINE_313 += ${BASE_TAR_PATH}-alpine3.13-x86_64-pwsh.tar.xz
TAR_XZ_ALPINE_313 += ${BASE_TAR_PATH}-alpine3.13-arm.tar.xz
TAR_XZ_ALPINE_313 += ${BASE_TAR_PATH}-alpine3.13-arm64.tar.xz
TAR_XZ_ALPINE_313 += ${BASE_TAR_PATH}-alpine3.13-s390x.tar.xz
TAR_XZ_ALPINE_313 += ${BASE_TAR_PATH}-alpine3.13-ppc64le.tar.xz

TAR_XZ_ALPINE_314 += ${BASE_TAR_PATH}-alpine3.14-x86_64.tar.xz
TAR_XZ_ALPINE_314 += ${BASE_TAR_PATH}-alpine3.14-arm.tar.xz
TAR_XZ_ALPINE_314 += ${BASE_TAR_PATH}-alpine3.14-arm64.tar.xz
TAR_XZ_ALPINE_314 += ${BASE_TAR_PATH}-alpine3.14-s390x.tar.xz
TAR_XZ_ALPINE_314 += ${BASE_TAR_PATH}-alpine3.14-ppc64le.tar.xz

TAR_XZ_ALPINE_315 += ${BASE_TAR_PATH}-alpine3.15-x86_64.tar.xz
TAR_XZ_ALPINE_315 += ${BASE_TAR_PATH}-alpine3.15-arm.tar.xz
TAR_XZ_ALPINE_315 += ${BASE_TAR_PATH}-alpine3.15-arm64.tar.xz
TAR_XZ_ALPINE_315 += ${BASE_TAR_PATH}-alpine3.15-s390x.tar.xz
TAR_XZ_ALPINE_315 += ${BASE_TAR_PATH}-alpine3.15-ppc64le.tar.xz

TAR_XZ_ALPINE_LATEST += ${BASE_TAR_PATH}-alpine-latest-x86_64.tar.xz
TAR_XZ_ALPINE_LATEST += ${BASE_TAR_PATH}-alpine-latest-arm.tar.xz
TAR_XZ_ALPINE_LATEST += ${BASE_TAR_PATH}-alpine-latest-arm64.tar.xz
TAR_XZ_ALPINE_LATEST += ${BASE_TAR_PATH}-alpine-latest-s390x.tar.xz
TAR_XZ_ALPINE_LATEST += ${BASE_TAR_PATH}-alpine-latest-ppc64le.tar.xz

TAR_XZ_UBUNTU += ${BASE_TAR_PATH}-ubuntu-x86_64.tar.xz
TAR_XZ_UBUNTU += ${BASE_TAR_PATH}-ubuntu-x86_64-pwsh.tar.xz
TAR_XZ_UBUNTU += ${BASE_TAR_PATH}-ubuntu-arm.tar.xz
TAR_XZ_UBUNTU += ${BASE_TAR_PATH}-ubuntu-arm64.tar.xz
TAR_XZ_UBUNTU += ${BASE_TAR_PATH}-ubuntu-s390x.tar.xz
TAR_XZ_UBUNTU += ${BASE_TAR_PATH}-ubuntu-ppc64le.tar.xz

TAR_XZ_FIPS += ${BASE_TAR_PATH}-ubi-fips-x86_64.tar.xz

# Binaries that we support for the helper image. We are using the following
# pattern match:
# out/binaries/gitlab-runner-helper/gitlab-runner-helper.{{arch}}-{{os}}, these should
# match up with GO_ARCH_* variables names. Note that Linux is implied by
# default.
BASE_BINARY_PATH := out/binaries/gitlab-runner-helper/gitlab-runner-helper
BINARIES := ${BASE_BINARY_PATH}.x86_64-windows
BINARIES += ${BASE_BINARY_PATH}.x86_64
BINARIES += ${BASE_BINARY_PATH}.arm
BINARIES += ${BASE_BINARY_PATH}.arm64
BINARIES += ${BASE_BINARY_PATH}.s390x
BINARIES += ${BASE_BINARY_PATH}.ppc64le

# Define variables with the architecture for each matching binary. We are using
# the following pattern match GO_ARCH_{{arch}}-{{os}}, these should match up
# with BINARIES variables. The value of the variable is the dist name from `go tool dist list`
GO_ARCH_x86_64 = linux/amd64
GO_ARCH_arm = linux/arm
GO_ARCH_arm64 = linux/arm64
GO_ARCH_s390x = linux/s390x
GO_ARCH_ppc64le = linux/ppc64le
GO_ARCH_x86_64-windows = windows/amd64

GO_ARCH_NAME_amd64 = x86_64

# Go files that are used to create the helper binary.
HELPER_GO_FILES ?= $(shell find common network -name '*.go')

ALPINE_312_VERSION ?= "3.12"
ALPINE_313_VERSION ?= "3.13"
ALPINE_314_VERSION ?= "3.14"
ALPINE_315_VERSION ?= "3.15"
UBUNTU_VERSION ?= "20.04"

# Build the Runner Helper binaries for the host platform.
.PHONY: helper-bin-host
helper-bin-host: ${BASE_BINARY_PATH}.$(shell uname -m)

# Build the Runner Helper binaries for all supported platforms.
.PHONY: helper-bin
helper-bin: $(BINARIES)

# Make sure the fips target is first since it's less general
${BASE_BINARY_PATH}-fips: export GOOS ?= linux
${BASE_BINARY_PATH}-fips: export GOARCH ?= amd64
${BASE_BINARY_PATH}-fips: APP_NAME := "gitlab-runner-helper"
${BASE_BINARY_PATH}-fips: $(HELPER_GO_FILES)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 go build \
    		   -tags fips \
    		   -ldflags "$(GO_LDFLAGS)" \
    		   -o="${BASE_BINARY_PATH}.$(GO_ARCH_NAME_$(GOARCH))-fips" \
    		   $(PKG)/apps/gitlab-runner-helper

${BASE_BINARY_PATH}-fips-docker: export GOOS ?= linux
${BASE_BINARY_PATH}-fips-docker: export GOARCH ?= amd64
${BASE_BINARY_PATH}-fips-docker: export GO_VERSION ?= 1.18
${BASE_BINARY_PATH}-fips-docker: $(HELPER_GO_FILES)
	# Building $(NAME)-helper in version $(VERSION) for FIPS $(GOOS) $(GOARCH)
	@docker build -t gitlab-runner-helper-fips --build-arg GOOS="$(GOOS)" --build-arg GOARCH="$(GOARCH)" --build-arg GO_VERSION="$(GO_VERSION)" -f dockerfiles/fips/helper.fips.Dockerfile .
	@docker rm -f gitlab-runner-helper-fips && docker create -it --name gitlab-runner-helper-fips gitlab-runner-helper-fips
	@docker cp gitlab-runner-helper-fips:/gitlab-runner-helper-fips "${BASE_BINARY_PATH}.$(GO_ARCH_NAME_$(GOARCH))-fips"
	@docker rm -f gitlab-runner-helper-fips

${BASE_BINARY_PATH}.%: APP_NAME := "gitlab-runner-helper"
${BASE_BINARY_PATH}.%: $(HELPER_GO_FILES) $(GOX)
	$(GOX) -osarch=$(GO_ARCH_$*) -ldflags "$(GO_LDFLAGS)" -output=$@ $(PKG)/apps/gitlab-runner-helper

# Build the Runner Helper tar files for host platform.
.PHONY: _helper-dockerarchive-host
_helper-dockerarchive-host: ${BASE_TAR_PATH}-$(IMAGE_TARGET_FLAVOUR)-$(shell uname -m)$(IMAGE_VARIANT_SUFFIX).tar.xz
	@ # NOTE: The ENTRYPOINT metadata is not preserved on export, so we need to reapply this metadata on import.
	@ # See https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2058#note_388341301
	docker import ${BASE_TAR_PATH}-$(IMAGE_TARGET_FLAVOUR)-$(shell uname -m)$(IMAGE_VARIANT_SUFFIX).tar.xz \
		--change "ENTRYPOINT [\"/usr/bin/dumb-init\", \"/entrypoint\"]" \
		gitlab/gitlab-runner-helper:$(IMAGE_VARIANT_PREFIX)$(shell uname -m)-$(REVISION)$(IMAGE_VARIANT_SUFFIX)

.PHONY: helper-dockerarchive-host
helper-dockerarchive-host:
	@$(MAKE) _helper-dockerarchive-host IMAGE_TARGET_FLAVOUR='alpine' IMAGE_VARIANT_PREFIX='' IMAGE_VARIANT_SUFFIX=''
	@$(MAKE) _helper-dockerarchive-host IMAGE_TARGET_FLAVOUR='alpine' IMAGE_VARIANT_PREFIX='' IMAGE_VARIANT_SUFFIX='-pwsh'
	@$(MAKE) _helper-dockerarchive-host IMAGE_TARGET_FLAVOUR='ubuntu' IMAGE_VARIANT_PREFIX='ubuntu-' IMAGE_VARIANT_SUFFIX=''
	@$(MAKE) _helper-dockerarchive-host IMAGE_TARGET_FLAVOUR='ubuntu' IMAGE_VARIANT_PREFIX='ubuntu-' IMAGE_VARIANT_SUFFIX='-pwsh'

# Build the Runner Helper tar files for all supported platforms.
.PHONY: helper-dockerarchive
helper-dockerarchive: helper-dockerarchive-alpine helper-dockerarchive-alpine3.13 helper-dockerarchive-alpine3.14 helper-dockerarchive-alpine3.15 helper-dockerarchive-alpine-latest helper-dockerarchive-ubuntu

.PHONY: helper-dockerarchive-alpine
helper-dockerarchive-alpine: $(TAR_XZ_ALPINE)

.PHONY: helper-dockerarchive-alpine3.13
helper-dockerarchive-alpine3.13: $(TAR_XZ_ALPINE_313)

.PHONY: helper-dockerarchive-alpine3.14
helper-dockerarchive-alpine3.14: $(TAR_XZ_ALPINE_314)

.PHONY: helper-dockerarchive-alpine3.15
helper-dockerarchive-alpine3.15: $(TAR_XZ_ALPINE_315)

.PHONY: helper-dockerarchive-alpine-latest
helper-dockerarchive-alpine-latest: $(TAR_XZ_ALPINE_LATEST)

.PHONY: helper-dockerarchive-ubuntu
helper-dockerarchive-ubuntu: $(TAR_XZ_UBUNTU)

.PHONY: helper-dockerarchive-ubi-fips
helper-dockerarchive-ubi-fips: $(TAR_XZ_FIPS)

${BASE_TAR_PATH}-ubi-fips-%.tar.xz: ${BASE_TAR_PATH}-ubi-fips-%.tar
	xz $(TAR_XZ_ARGS) $<

${BASE_TAR_PATH}-%-pwsh.tar.xz: ${BASE_TAR_PATH}-%-pwsh.tar
	xz $(TAR_XZ_ARGS) $<

${BASE_TAR_PATH}-%.tar.xz: ${BASE_TAR_PATH}-%.tar
	xz $(TAR_XZ_ARGS) $<

${BASE_TAR_PATH}-ubi-fips-%.tar: export TARGET_DOCKERFILE ?= Dockerfile.fips
${BASE_TAR_PATH}-ubi-fips-%.tar: export HELPER_BINARY_POSTFIX ?= -fips
${BASE_TAR_PATH}-ubi-fips-%.tar:
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker redhat/ubi8 $* $@ $(UBI_FIPS_VERSION)

# See https://github.com/PowerShell/powershell/releases for values of PWSH_VERSION/PWSH_IMAGE_DATE
${BASE_TAR_PATH}-alpine-%-pwsh.tar: export IMAGE_SHELL := pwsh
${BASE_TAR_PATH}-alpine-%-pwsh.tar: export PWSH_VERSION ?= 7.1.1
${BASE_TAR_PATH}-alpine-%-pwsh.tar: export PWSH_IMAGE_DATE ?= 20210114
${BASE_TAR_PATH}-alpine-%-pwsh.tar: ${BASE_BINARY_PATH}.%
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker alpine $* $@ 3.12

${BASE_TAR_PATH}-alpine3.13-%-pwsh.tar: export IMAGE_SHELL := pwsh
${BASE_TAR_PATH}-alpine3.13-%-pwsh.tar: export PWSH_VERSION ?= 7.1.4
${BASE_TAR_PATH}-alpine3.13-%-pwsh.tar: export PWSH_IMAGE_DATE ?= 20210927
${BASE_TAR_PATH}-alpine3.13-%-pwsh.tar: ${BASE_BINARY_PATH}.%
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker alpine $* $@ 3.13

${BASE_TAR_PATH}-ubuntu-%-pwsh.tar: export IMAGE_SHELL := pwsh
${BASE_TAR_PATH}-ubuntu-%-pwsh.tar: export PWSH_VERSION ?= 7.1.1
${BASE_TAR_PATH}-ubuntu-%-pwsh.tar: export PWSH_IMAGE_DATE ?= 20210114
${BASE_TAR_PATH}-ubuntu-%-pwsh.tar: ${BASE_BINARY_PATH}.%
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker ubuntu $* $@ 20.04

${BASE_TAR_PATH}-alpine-latest-%.tar: ${BASE_BINARY_PATH}.%
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker alpine $* $@ latest

${BASE_TAR_PATH}-alpine-%.tar: ${BASE_BINARY_PATH}.%
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker alpine $* $@ $(ALPINE_312_VERSION)

${BASE_TAR_PATH}-alpine3.13-%.tar: ${BASE_BINARY_PATH}.%
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker alpine $* $@ $(ALPINE_313_VERSION)

${BASE_TAR_PATH}-alpine3.14-%.tar: ${BASE_BINARY_PATH}.%
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker alpine $* $@ $(ALPINE_314_VERSION)

${BASE_TAR_PATH}-alpine3.15-%.tar: ${BASE_BINARY_PATH}.%
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker alpine $* $@ $(ALPINE_315_VERSION)

${BASE_TAR_PATH}-ubuntu-%.tar: ${BASE_BINARY_PATH}.%
	@mkdir -p $$(dirname $@_)
	@./ci/build_helper_docker ubuntu $* $@ $(UBUNTU_VERSION)
