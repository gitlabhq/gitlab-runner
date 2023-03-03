runner-bin: $(GOX)
	# Building $(NAME) in version $(VERSION) for $(BUILD_PLATFORMS)
	$(GOX) $(BUILD_PLATFORMS) \
		   -ldflags "$(GO_LDFLAGS)" \
		   -output="out/binaries/$(NAME)-{{.OS}}-{{.Arch}}" \
		   $(PKG)

runner-bin-fips: export GOOS ?= linux
runner-bin-fips: export GOARCH ?= amd64
runner-bin-fips:
	# Building $(NAME) in version $(VERSION) for FIPS $(GOOS) $(GOARCH)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 go build \
		   -tags fips \
		   -ldflags "$(GO_LDFLAGS)" \
		   -o="out/binaries/$(NAME)-$(GOOS)-$(GOARCH)-fips" \
		   $(PKG)

go-fips-docker: export GO_VERSION ?= 1.18
go-fips-docker: export UBI_VERSION ?= $(UBI_FIPS_VERSION)
go-fips-docker: export BUILD_IMAGE ?= registry.gitlab.com/gitlab-org/gitlab-runner/go-fips:$(GO_VERSION)
go-fips-docker: export BUILD_DOCKERFILE ?= ./dockerfiles/ci/go.fips.Dockerfile
go-fips-docker:
	# Building Go FIPS Docker image
	@./ci/build_go_fips_image

runner-bin-fips-docker: export GO_VERSION ?= 1.18
runner-bin-fips-docker: export GOOS ?= linux
runner-bin-fips-docker: export GOARCH ?= amd64
runner-bin-fips-docker: export BUILD_IMAGE ?= go-fips
runner-bin-fips-docker:
	# Building $(NAME) in version $(VERSION) for FIPS $(GOOS) $(GOARCH)
	docker build -t gitlab-runner-fips --build-arg GOOS="$(GOOS)" --build-arg GOARCH="$(GOARCH)" --build-arg GO_VERSION="$(GO_VERSION)" --build-arg BUILD_IMAGE="$(BUILD_IMAGE)" -f dockerfiles/fips/runner.fips.Dockerfile .
	@docker rm -f gitlab-runner-fips && docker create -it --name gitlab-runner-fips gitlab-runner-fips
	@docker cp gitlab-runner-fips:/gitlab-runner-linux-amd64-fips out/binaries/
	@docker rm -f gitlab-runner-fips

runner-bin-host: OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
runner-bin-host: ARCH := $(shell uname -m | sed 's/aarch64/arm64/ ; s/x86_64/amd64/ ; s/i386/386/')
runner-bin-host:
	# Building $(NAME) in version $(VERSION) for host platform
	$(MAKE) runner-bin BUILD_PLATFORMS="-osarch=$(OS)/$(ARCH)"
	cp -f "out/binaries/$(NAME)-$(OS)-$(ARCH)" out/binaries/gitlab-runner

runner-bin-linux: OS := 'linux'
runner-bin-linux:
	$(MAKE) runner-bin BUILD_PLATFORMS="-os=$(OS) $(BUILD_ARCHS)"

runner-and-helper-bin-host: runner-bin-host helper-bin-host helper-dockerarchive-host

runner-and-helper-bin-linux: runner-bin-linux helper-dockerarchive

runner-and-helper-bin: runner-bin helper-bin helper-dockerarchive

runner-and-helper-docker-host: export CI_COMMIT_REF_SLUG=$(shell echo $(BRANCH) | cut -c -63 | sed -E 's/[^a-z0-9-]+/-/g' | sed -E 's/^-*([a-z0-9-]+[a-z0-9])-*$$/\1/g')
runner-and-helper-docker-host: runner-and-helper-deb-host
	$(MAKE) release_docker_images
	$(MAKE) release_helper_docker_images

runner-and-helper-deb-host: ARCH := $(shell uname -m | sed 's/aarch64/arm64/; s/x86_64/amd64/; s/i386/386/')
runner-and-helper-deb-host: export BUILD_ARCHS := -arch '$(ARCH)'
runner-and-helper-deb-host: PACKAGE_ARCH := $(shell uname -m | sed 's/aarch64/arm64/; s/x86_64/amd64/; s/i386/i686/')
runner-and-helper-deb-host: runner-and-helper-bin-host package-deps package-prepare
	$(MAKE) package-deb-arch ARCH=$(ARCH) PACKAGE_ARCH=$(PACKAGE_ARCH)

runner-and-helper-rpm-host: ARCH := $(shell uname -m | sed 's/aarch64/arm64/; s/x86_64/amd64/; s/i386/386/')
runner-and-helper-rpm-host: export BUILD_ARCHS := -arch '$(ARCH)'
runner-and-helper-rpm-host: PACKAGE_ARCH := $(shell uname -m | sed 's/aarch64/arm64/; s/x86_64/amd64/; s/i386/i686/')
runner-and-helper-rpm-host: runner-and-helper-bin-host package-deps package-prepare
	$(MAKE) package-rpm-arch ARCH=$(ARCH) PACKAGE_ARCH=$(PACKAGE_ARCH)

UNIX_ARCHS_CHECK ?= aix/ppc64 android/amd64 dragonfly/amd64 freebsd/amd64 hurd/amd64 illumos/amd64 linux/riscv64 netbsd/amd64 openbsd/amd64 solaris/amd64

# runner-unix-check compiles against various unix OSs that we don't officially support. This is not used
# as part of any CI job at the moment, but is to be used locally to easily determine what currently compiles.
runner-unix-check:
	$(MAKE) $(foreach OSARCH,$(UNIX_ARCHS_CHECK),runner-unix-check-arch-$(subst /,-,$(OSARCH)))

runner-unix-check-arch-%:
	GOOS=$(subst -, GOARCH=,$(subst runner-unix-check-arch-,,$@)) go build -o /dev/null || true
