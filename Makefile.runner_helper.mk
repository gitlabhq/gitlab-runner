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

# Binaries that we support for the helper image. We are using the following
# pattern match:
# out/binaries/gitlab-runner-helper/gitlab-runner-helper.{{os}}-{{arch}}
BASE_BINARY_PATH := out/binaries/gitlab-runner-helper/gitlab-runner-helper
BINARIES := ${BASE_BINARY_PATH}.windows-amd64.exe
BINARIES += ${BASE_BINARY_PATH}.linux-amd64
BINARIES += ${BASE_BINARY_PATH}.linux-arm
BINARIES += ${BASE_BINARY_PATH}.linux-arm64
BINARIES += ${BASE_BINARY_PATH}.linux-s390x
BINARIES += ${BASE_BINARY_PATH}.linux-ppc64le
BINARIES += ${BASE_BINARY_PATH}.linux-riscv64
BINARIES += ${BASE_BINARY_PATH}.linux-amd64-fips

# Go files that are used to create the helper binary.
HELPER_GO_FILES ?= $(shell find common network -name '*.go')

# Build the Runner Helper binaries for the host platform.
.PHONY: helper-bin-host
helper-bin-host: ${BASE_BINARY_PATH}.$(shell go env GOOS)-$(shell go env GOARCH)

# Build the Runner Helper binaries for the linux OS and host architecture.
.PHONY: helper-bin-linux
helper-bin-linux: ${BASE_BINARY_PATH}.linux-$(shell go env GOARCH)

# Build the Runner Helper binaries for all supported platforms.
.PHONY: helper-bin
helper-bin: $(BINARIES)

.PHONY: helper-bin-fips
helper-bin-fips: ${BASE_BINARY_PATH}.linux-amd64-fips

.PHONY: helper-images
helper-images: $(BINARIES)
helper-images: out/helper-images

.PHONY: helper-image-host
helper-image-host: export HOST_ARCH ?= $(shell go env GOARCH)
helper-image-host: export HOST_FLAVOR ?= alpine-3.21
helper-image-host: export RUNNER_IMAGES_VERSION ?= $(shell grep "RUNNER_IMAGES_VERSION:" .gitlab/ci/_common.gitlab-ci.yml | awk -F': ' '{ print $$2 }' | tr -d '"')
helper-image-host: helper-bin-linux
	docker buildx create --name builder --use --driver docker-container default || true
	mkdir -p out/helper-images
	cd dockerfiles/runner-helper && docker buildx bake --progress plain host-image

# Make sure the fips target is first since it's less general
${BASE_BINARY_PATH}.linux-amd64-fips: GOOS=linux
${BASE_BINARY_PATH}.linux-amd64-fips: GOARCH=amd64
${BASE_BINARY_PATH}.linux-amd64-fips: APP_NAME := "gitlab-runner-helper"
${BASE_BINARY_PATH}.linux-amd64-fips: $(HELPER_GO_FILES)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 GOEXPERIMENT=boringcrypto go build -tags fips -trimpath -ldflags "$(GO_LDFLAGS)" -o $@ $(PKG)/apps/gitlab-runner-helper

$(BASE_BINARY_PATH)-%: GOOS=$(firstword $(subst -, ,$*))
$(BASE_BINARY_PATH)-%: GOARCH=$(lastword $(subst -, ,$(basename $*)))
$(BASE_BINARY_PATH)-%: APP_NAME := "gitlab-runner-helper"
${BASE_BINARY_PATH}.%: $(HELPER_GO_FILES)
	GOOS="$(GOOS)" GOARCH="$(GOARCH)" go build -trimpath -ldflags "$(GO_LDFLAGS)" -o $@ $(PKG)/apps/gitlab-runner-helper

out/helper-images: TARGETS ?= alpine alpine-pwsh ubuntu ubuntu-pwsh
out/helper-images:
	docker buildx create --name builder --use --driver docker-container default || true
	mkdir -p out/helper-images
	cd dockerfiles/runner-helper && docker buildx bake --progress plain $(TARGETS)

.PHONY: prebuilt-helper-images
prebuilt-helper-images: ALPINE_DEFAULT_VERSION=3.21
prebuilt-helper-images:
	@find out/helper-images -maxdepth 1 -name "*.tar" | parallel -j$(shell nproc) './ci/prebuilt_helper_image {}'

	@for file in out/helper-images/prebuilt-alpine$(ALPINE_DEFAULT_VERSION)-*.tar.xz; do \
		target=$$(echo -n "$${file}" | sed -e 's/'$(ALPINE_DEFAULT_VERSION)'//'); \
		if [ ! -e "$$target" ]; then \
			ln -s "$$(basename $$file)" "$$target"; \
		fi; \
	done
