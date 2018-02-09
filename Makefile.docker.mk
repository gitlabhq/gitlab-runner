docker: out/docker/prebuilt-x86_64.tar.xz out/docker/prebuilt-arm.tar.xz

HELPER_GO_FILES ?= $(shell find common network vendor -name '*.go')

out/docker/prebuilt-x86_64.tar.xz: $(HELPER_GO_FILES) $(GOX)
	# Create directory
	mkdir -p out/docker

ifneq (, $(shell docker info))
	# Building gitlab-runner-helper
	gox -osarch=linux/amd64 \
		-ldflags "$(GO_LDFLAGS)" \
		-output="dockerfiles/build/gitlab-runner-helper" \
		$(PKG)/apps/gitlab-runner-helper

	# Build docker images
	docker build -t gitlab/gitlab-runner-helper:x86_64-$(REVISION) -f dockerfiles/build/Dockerfile.x86_64 dockerfiles/build
	-docker rm -f gitlab-runner-prebuilt-x86_64-$(REVISION)
	docker create --name=gitlab-runner-prebuilt-x86_64-$(REVISION) gitlab/gitlab-runner-helper:x86_64-$(REVISION) /bin/sh
	docker export -o out/docker/prebuilt-x86_64.tar gitlab-runner-prebuilt-x86_64-$(REVISION)
	docker rm -f gitlab-runner-prebuilt-x86_64-$(REVISION)
	xz -f -9 out/docker/prebuilt-x86_64.tar
else
	$(warning =============================================)
	$(warning WARNING: downloading prebuilt docker images that will be embedded in gitlab-runner)
	$(warning WARNING: to use images compiled from your code install Docker Engine)
	$(warning WARNING: and remove out/docker/prebuilt-x86_64.tar.xz)
	$(warning =============================================)
	curl -o out/docker/prebuilt-x86_64.tar.xz \
		https://gitlab-runner-downloads.s3.amazonaws.com/master/docker/prebuilt-x86_64.tar.xz
endif

out/docker/prebuilt-arm.tar.xz: $(HELPER_GO_FILES) $(GOX)
	# Create directory
	mkdir -p out/docker

ifneq (, $(shell docker info))
	# Building gitlab-runner-helper
	gox -osarch=linux/arm \
		-ldflags "$(GO_LDFLAGS)" \
		-output="dockerfiles/build/gitlab-runner-helper" \
		$(PKG)/apps/gitlab-runner-helper

	# Build docker images
	docker build -t gitlab/gitlab-runner-helper:arm-$(REVISION) -f dockerfiles/build/Dockerfile.arm dockerfiles/build
	-docker rm -f gitlab-runner-prebuilt-arm-$(REVISION)
	docker create --name=gitlab-runner-prebuilt-arm-$(REVISION) gitlab/gitlab-runner-helper:arm-$(REVISION) /bin/sh
	docker export -o out/docker/prebuilt-arm.tar gitlab-runner-prebuilt-arm-$(REVISION)
	docker rm -f gitlab-runner-prebuilt-arm-$(REVISION)
	xz -f -9 out/docker/prebuilt-arm.tar
else
	$(warning =============================================)
	$(warning WARNING: downloading prebuilt docker images that will be embedded in gitlab-runner)
	$(warning WARNING: to use images compiled from your code install Docker Engine)
	$(warning WARNING: and remove out/docker/prebuilt-arm.tar.xz)
	$(warning =============================================)
	curl -o out/docker/prebuilt-arm.tar.xz \
		https://gitlab-runner-downloads.s3.amazonaws.com/master/docker/prebuilt-arm.tar.xz
endif

