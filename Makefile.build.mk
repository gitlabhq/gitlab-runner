runner-bin: $(GOX)
	# Building $(NAME) in version $(VERSION) for $(BUILD_PLATFORMS)
	$(GOX) $(BUILD_PLATFORMS) \
		   -ldflags "$(GO_LDFLAGS)" \
		   -output="out/binaries/$(NAME)-{{.OS}}-{{.Arch}}" \
		   $(PKG)

runner-bin-host: OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
runner-bin-host: ARCH := $(shell uname -m | sed s/x86_64/amd64/ | sed s/i386/386/)
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

runner-and-helper-deb-host: ARCH := $(shell uname -m | sed s/x86_64/amd64/ | sed s/i386/386/)
runner-and-helper-deb-host: export BUILD_ARCHS := -arch '$(ARCH)'
runner-and-helper-deb-host: PACKAGE_ARCH := $(shell uname -m | sed s/x86_64/amd64/ | sed s/i386/i686/)
runner-and-helper-deb-host: runner-and-helper-bin-host package-deps package-prepare
	$(MAKE) package-deb-arch ARCH=$(ARCH) PACKAGE_ARCH=$(PACKAGE_ARCH)

runner-and-helper-rpm-host: ARCH := $(shell uname -m | sed s/x86_64/amd64/ | sed s/i386/386/)
runner-and-helper-rpm-host: export BUILD_ARCHS := -arch '$(ARCH)'
runner-and-helper-rpm-host: PACKAGE_ARCH := $(shell uname -m | sed s/x86_64/amd64/ | sed s/i386/i686/)
runner-and-helper-rpm-host: runner-and-helper-bin-host package-deps package-prepare
	$(MAKE) package-rpm-arch ARCH=$(ARCH) PACKAGE_ARCH=$(PACKAGE_ARCH)
