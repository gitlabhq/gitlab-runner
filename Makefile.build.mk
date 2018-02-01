export RUNNER_BINARY ?= out/binaries/$(NAME)

build: $(GOX)
	# Building $(NAME) in version $(VERSION) for $(BUILD_PLATFORMS)
	gox $(BUILD_PLATFORMS) \
		-ldflags "$(GO_LDFLAGS)" \
		-output="out/binaries/$(NAME)-{{.OS}}-{{.Arch}}" \
		$(PKG)

build_simple: $(GOPATH_SETUP)
	# Building $(NAME) in version $(VERSION) for current platform
	go build \
		-ldflags "$(GO_LDFLAGS)" \
		-o "$(RUNNER_BINARY)" \
		$(PKG)

build_current: helper-docker build_simple

build_current_docker: build_current_deb
	make release_docker_images RUNNER_BINARY=$(RUNNER_BINARY)

build_current_deb: build_current package-deps package-prepare
	make package-deb-fpm ARCH=amd64 PACKAGE_ARCH=amd64 RUNNER_BINARY=$(RUNNER_BINARY)

build_current_rpm: build_current package-deps package-prepare
	make package-rpm-fpm ARCH=amd64 PACKAGE_ARCH=amd64 RUNNER_BINARY=$(RUNNER_BINARY)
