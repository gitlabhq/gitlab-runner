ifeq (, $(shell docker info 2>/dev/null))
USE_PRECOMPILED_IMAGES ?= 1
endif

docker: out/helper-images/prebuilt-x86_64.tar.xz out/helper-images/prebuilt-arm.tar.xz

ifeq (, $(USE_PRECOMPILED_IMAGES))
HELPER_GO_FILES ?= $(shell find common network vendor -name '*.go')

GO_x86_64_ARCH = amd64
GO_arm_ARCH = arm

dockerfiles/build/binaries/gitlab-runner-helper.%: $(HELPER_GO_FILES) $(GOX)
	gox -osarch=linux/$(GO_$*_ARCH) -ldflags "$(GO_LDFLAGS)" -output=$@ $(PKG)/apps/gitlab-runner-helper

out/helper-images/prebuilt-%.tar: dockerfiles/build/binaries/gitlab-runner-helper.%
	@mkdir -p $$(dirname $@_)
	docker build -t gitlab/gitlab-runner-helper:$*-$(REVISION) -f dockerfiles/build/Dockerfile.$* dockerfiles/build
	-docker rm -f gitlab-runner-prebuilt-$*-$(REVISION)
	docker create --name=gitlab-runner-prebuilt-$*-$(REVISION) gitlab/gitlab-runner-helper:$*-$(REVISION) /bin/sh
	docker export -o $@ gitlab-runner-prebuilt-$*-$(REVISION)
	docker rm -f gitlab-runner-prebuilt-$*-$(REVISION)

out/helper-images/prebuilt-%.tar.xz: out/helper-images/prebuilt-%.tar
	xz -f -9 $<

else

out/helper-images/prebuilt-%.tar.xz:
	$(warning WARNING: downloading prebuilt docker images that will be loaded by gitlab-runner: $@)
	@mkdir -p $$(dirname $@_)
	curl -o $@ https://gitlab-runner-downloads.s3.amazonaws.com/master/docker/$(shell basename $@)
endif
