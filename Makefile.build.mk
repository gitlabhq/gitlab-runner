BASE_BINARY_PATH := out/binaries/$(NAME)
BINARIES := ${BASE_BINARY_PATH}-linux-amd64
BINARIES += ${BASE_BINARY_PATH}-linux-arm64
BINARIES += ${BASE_BINARY_PATH}-linux-386
BINARIES += ${BASE_BINARY_PATH}-linux-arm
BINARIES += ${BASE_BINARY_PATH}-linux-s390x
BINARIES += ${BASE_BINARY_PATH}-linux-ppc64le
BINARIES += ${BASE_BINARY_PATH}-linux-riscv64
BINARIES += ${BASE_BINARY_PATH}-darwin-amd64
BINARIES += ${BASE_BINARY_PATH}-darwin-arm64
BINARIES += ${BASE_BINARY_PATH}-freebsd-386
BINARIES += ${BASE_BINARY_PATH}-freebsd-amd64
BINARIES += ${BASE_BINARY_PATH}-freebsd-arm
BINARIES += ${BASE_BINARY_PATH}-windows-386.exe
BINARIES += ${BASE_BINARY_PATH}-windows-amd64.exe


.PHONY: runner-bin
runner-bin: $(BINARIES)

.PHONY: runner-bin-fips
runner-bin-fips: $(BASE_BINARY_PATH)-linux-amd64-fips

.PHONY: runner-images
runner-images: $(BINARIES)
runner-images: out/runner-images

$(BASE_BINARY_PATH)-linux-amd64-fips: GOOS=linux
$(BASE_BINARY_PATH)-linux-amd64-fips: GOARCH=amd64
$(BASE_BINARY_PATH)-linux-amd64-fips:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 GOEXPERIMENT=boringcrypto go build -tags fips -ldflags "$(GO_LDFLAGS)" -o $@

$(BASE_BINARY_PATH)-%: GOOS=$(firstword $(subst -, ,$*))
$(BASE_BINARY_PATH)-%: GOARCH=$(lastword $(subst -, ,$(basename $*)))
$(BASE_BINARY_PATH)-%:
	GOOS="$(GOOS)" GOARCH="$(GOARCH)" go build -trimpath -ldflags "$(GO_LDFLAGS)" -o $@

.PHONY: runner-image-host
runner-image-host: export HOST_ARCH ?= $(shell go env GOARCH)
runner-image-host: export HOST_FLAVOR ?= alpine-3.21
runner-image-host: export RUNNER_IMAGES_VERSION ?= $(shell grep "RUNNER_IMAGES_VERSION:" .gitlab/ci/_common.gitlab-ci.yml | awk -F': ' '{ print $$2 }' | tr -d '"')
runner-image-host: runner-bin-linux
	cd dockerfiles/runner && docker buildx bake --progress plain host-image

.PHONY: runner-and-helper-image-host
runner-and-helper-image-host: runner-image-host helper-image-host

out/runner-images: TARGETS ?= ubuntu alpine
out/runner-images:
	docker buildx create --name builder --use --driver docker-container default || true
	mkdir -p out/runner-images
	cd dockerfiles/runner && docker buildx bake --progress plain $(TARGETS)

ARCH_REPLACE="s/aarch64/arm64/ ; s/armv7l/arm/ ; s/x86_64/amd64/ ; s/i386/386/"

runner-bin-host: OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
runner-bin-host: ARCH := $(shell uname -m | sed $(ARCH_REPLACE))
runner-bin-host:
	$(MAKE) ${BASE_BINARY_PATH}-${OS}-$(ARCH)

runner-bin-linux: OS := 'linux'
runner-bin-linux: ARCH := $(shell uname -m | sed $(ARCH_REPLACE))
runner-bin-linux:
	$(MAKE) ${BASE_BINARY_PATH}-${OS}-$(ARCH)

runner-and-helper-bin-host: runner-bin-host helper-bin-host

runner-and-helper-bin-linux: runner-bin-linux helper-images prebuilt-helper-images

runner-and-helper-bin: runner-bin helper-images prebuilt-helper-images

runner-and-helper-deb-host: ARCH := $(shell uname -m | sed $(ARCH_REPLACE))
runner-and-helper-deb-host: export BUILD_ARCHS := -arch '$(ARCH)'
runner-and-helper-deb-host: PACKAGE_ARCH := $(shell uname -m | sed $(ARCH_REPLACE))
runner-and-helper-deb-host: runner-and-helper-bin-host
	$(MAGE) package:deps package:prepare
	$(MAKE) package-deb-arch ARCH=$(ARCH) PACKAGE_ARCH=$(PACKAGE_ARCH)

runner-and-helper-rpm-host: ARCH := $(shell uname -m | sed $(ARCH_REPLACE))
runner-and-helper-rpm-host: export BUILD_ARCHS := -arch '$(ARCH)'
runner-and-helper-rpm-host: PACKAGE_ARCH := $(shell uname -m | sed $(ARCH_REPLACE))
runner-and-helper-rpm-host: runner-and-helper-bin-host
	$(MAGE) package:deps package:prepare
	$(MAKE) package-rpm-arch ARCH=$(ARCH) PACKAGE_ARCH=$(PACKAGE_ARCH)

UNIX_ARCHS_CHECK ?= aix/ppc64 android/amd64 dragonfly/amd64 freebsd/amd64 hurd/amd64 illumos/amd64 linux/riscv64 netbsd/amd64 openbsd/amd64 solaris/amd64

# runner-unix-check compiles against various unix OSs that we don't officially support. This is not used
# as part of any CI job at the moment, but is to be used locally to easily determine what currently compiles.
runner-unix-check:
	$(MAKE) $(foreach OSARCH,$(UNIX_ARCHS_CHECK),runner-unix-check-arch-$(subst /,-,$(OSARCH)))

runner-unix-check-arch-%:
	GOOS=$(subst -, GOARCH=,$(subst runner-unix-check-arch-,,$@)) go build -o /dev/null || true
