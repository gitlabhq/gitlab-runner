NAME ?= gitlab-runner
PACKAGE_NAME ?= $(NAME)
export VERSION := $(shell ./ci/version)
REVISION := $(shell git rev-parse --short=8 HEAD || echo unknown)
BRANCH := $(shell git show-ref | grep "$(REVISION)" | grep -v HEAD | awk '{print $$2}' | sed 's|refs/remotes/origin/||' | sed 's|refs/heads/||' | sort | head -n 1)
BUILT := $(shell date +%Y-%m-%dT%H:%M:%S%:z)
TESTFLAGS ?= -cover

LATEST_STABLE_TAG := $(shell git -c versionsort.prereleaseSuffix="-rc" -c versionsort.prereleaseSuffix="-RC" tag -l "v*.*.*" --sort=-v:refname | awk '!/rc/' | head -n 1)
export IS_LATEST :=
ifeq ($(shell git describe --exact-match --match $(LATEST_STABLE_TAG) >/dev/null 2>&1; echo $$?), 0)
export IS_LATEST := true
endif

PACKAGE_CLOUD ?= ayufan/gitlab-ci-multi-runner
PACKAGE_CLOUD_URL ?= https://packagecloud.io/
BUILD_PLATFORMS ?= -os '!netbsd' -os '!openbsd'
S3_UPLOAD_PATH ?= master
# Keep in sync with docs/install/linux-repository.md
DEB_PLATFORMS ?= debian/wheezy debian/jessie debian/stretch debian/buster \
    ubuntu/trusty ubuntu/xenial ubuntu/zesty ubuntu/artful \
    raspbian/wheezy raspbian/jessie raspbian/stretch raspbian/buster \
    linuxmint/qiana linuxmint/rebecca linuxmint/rafaela linuxmint/rosa linuxmint/sarah linuxmint/serena linuxmint/sonya
DEB_ARCHS ?= amd64 i386 armel armhf
RPM_PLATFORMS ?= el/6 el/7 \
    ol/6 ol/7 \
    fedora/25 fedora/26
RPM_ARCHS ?= x86_64 i686 arm armhf

PKG = gitlab.com/gitlab-org/$(PACKAGE_NAME)
COMMON_PACKAGE_NAMESPACE=$(PKG)/common

BUILD_DIR := $(CURDIR)
TARGET_DIR := $(BUILD_DIR)/out

ORIGINAL_GOPATH = $(shell echo $$GOPATH)
LOCAL_GOPATH := $(CURDIR)/.gopath
GOPATH_SETUP := $(LOCAL_GOPATH)/.ok
GOPATH_BIN := $(LOCAL_GOPATH)/bin
PKG_BUILD_DIR := $(LOCAL_GOPATH)/src/$(PKG)

export GOPATH = $(LOCAL_GOPATH)
export PATH := $(GOPATH_BIN):$(PATH)

# Packages in vendor/ are included in ./...
# https://github.com/golang/go/issues/11659
OUR_PACKAGES ?= $(subst _$(BUILD_DIR),$(PKG),$(shell go list ./... | grep -v '/vendor/'))

GO_LDFLAGS ?= -X $(COMMON_PACKAGE_NAMESPACE).NAME=$(PACKAGE_NAME) -X $(COMMON_PACKAGE_NAMESPACE).VERSION=$(VERSION) \
              -X $(COMMON_PACKAGE_NAMESPACE).REVISION=$(REVISION) -X $(COMMON_PACKAGE_NAMESPACE).BUILT=$(BUILT) \
              -X $(COMMON_PACKAGE_NAMESPACE).BRANCH=$(BRANCH) \
              -s -w
GO_FILES ?= $(shell find . -name '*.go' | grep -v './.gopath/')
export CGO_ENABLED ?= 0


# Development Tools
GOX = $(GOPATH_BIN)/gox
GOBINDATA = $(GOPATH_BIN)/go-bindata
MOCKERY = $(GOPATH_BIN)/mockery
DEVELOPMENT_TOOLS = $(GOX) $(GOBINDATA) $(MOCKERY)

MOCKERY_FLAGS = -note="This comment works around https://github.com/vektra/mockery/issues/155"

.PHONY: clean version mocks

all: deps build

help:
	# Commands:
	# make all => deps build
	# make version - show information about current version
	#
	# Development commands:
	# make install - install the version suitable for your OS as gitlab-runner
	# make docker - build docker dependencies
	#
	# Testing commands:
	# make test - run project tests
	# make codequality - run code quality analysis
	#
	# Deployment commands:
	# make deps - install all dependencies
	# make build - build project for all supported OSes
	# make package - package project using FPM
	# make packagecloud - send all packages to packagecloud
	# make packagecloud-yank - remove specific version from packagecloud

version:
	@echo Current version: $(VERSION)
	@echo Current revision: $(REVISION)
	@echo Current branch: $(BRANCH)
	@echo Build platforms: $(BUILD_PLATFORMS)
	@echo DEB platforms: $(DEB_PLATFORMS)
	@echo RPM platforms: $(RPM_PLATFORMS)
	@echo IS_LATEST: $(IS_LATEST)

deps: $(DEVELOPMENT_TOOLS)

codequality:
	./scripts/codequality analyze --dev

out/docker/prebuilt-x86_64.tar.xz: $(GO_FILES) $(GOX)
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

out/docker/prebuilt-arm.tar.xz: $(GO_FILES) $(GOX)
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

executors/docker/bindata.go: out/docker/prebuilt-x86_64.tar.xz out/docker/prebuilt-arm.tar.xz $(GOBINDATA)
	# Generating embedded data
	go-bindata \
		-pkg docker \
		-nocompress \
		-nomemcopy \
		-nometadata \
		-prefix out/docker/ \
		-o executors/docker/bindata.go \
		out/docker/prebuilt-x86_64.tar.xz \
		out/docker/prebuilt-arm.tar.xz
	go fmt executors/docker/bindata.go

docker: executors/docker/bindata.go

build: executors/docker/bindata.go $(GOX)
	# Building $(NAME) in version $(VERSION) for $(BUILD_PLATFORMS)
	gox $(BUILD_PLATFORMS) \
		-ldflags "$(GO_LDFLAGS)" \
		-output="out/binaries/$(NAME)-{{.OS}}-{{.Arch}}" \
		$(PKG)

build_simple: $(GOPATH_SETUP)
	# Building $(NAME) in version $(VERSION) for current platform
	go build \
		-ldflags "$(GO_LDFLAGS)" \
		-o "out/binaries/$(NAME)" \
		$(PKG)

build_current: executors/docker/bindata.go build_simple

check_race_conditions: executors/docker/bindata.go
	@./scripts/check_race_conditions $(OUR_PACKAGES)

test: executors/docker/bindata.go
	# Running tests...
	go test $(OUR_PACKAGES) $(TESTFLAGS)

install: executors/docker/bindata.go
	go install --ldflags="$(GO_LDFLAGS)" $(PKG)

dockerfiles:
	make -C dockerfiles all

# We rely on user GOPATH 'cause mockery seems not to be able to find dependencies in vendor directory
mocks: $(MOCKERY)
	rm -rf ./helpers/service/mocks
	find . -type f ! -path '*vendor/*' -name 'mock_*' -delete
	GOPATH=$(ORIGINAL_GOPATH) mockery $(MOCKERY_FLAGS) -dir=./vendor/github.com/ayufan/golang-kardianos-service -output=./helpers/service/mocks -name='(Interface|Logger)'
	GOPATH=$(ORIGINAL_GOPATH) mockery $(MOCKERY_FLAGS) -dir=./helpers/docker -all -inpkg
	GOPATH=$(ORIGINAL_GOPATH) mockery $(MOCKERY_FLAGS) -dir=./common -all -inpkg
	GOPATH=$(ORIGINAL_GOPATH) mockery $(MOCKERY_FLAGS) -dir=./shells -name fakeIAMCredentialsProvider -inpkg

test-docker:
	make test-docker-image IMAGE=centos:6 TYPE=rpm
	make test-docker-image IMAGE=centos:7 TYPE=rpm
	make test-docker-image IMAGE=debian:wheezy TYPE=deb
	make test-docker-image IMAGE=debian:jessie TYPE=deb
	make test-docker-image IMAGE=ubuntu-upstart:precise TYPE=deb
	make test-docker-image IMAGE=ubuntu-upstart:trusty TYPE=deb
	make test-docker-image IMAGE=ubuntu-upstart:utopic TYPE=deb

test-docker-image:
	tests/test_installation.sh $(IMAGE) out/$(TYPE)/$(PACKAGE_NAME)_amd64.$(TYPE)
	tests/test_installation.sh $(IMAGE) out/$(TYPE)/$(PACKAGE_NAME)_amd64.$(TYPE) Y

build-and-deploy:
	make build BUILD_PLATFORMS="-os=linux -arch=amd64"
	make package-deb-fpm ARCH=amd64 PACKAGE_ARCH=amd64
	scp out/deb/$(PACKAGE_NAME)_amd64.deb $(SERVER):
	ssh $(SERVER) dpkg -i $(PACKAGE_NAME)_amd64.deb

build-and-deploy-binary:
	make build BUILD_PLATFORMS="-os=linux -arch=amd64"
	scp out/binaries/$(PACKAGE_NAME)-linux-amd64 $(SERVER):/usr/bin/gitlab-runner

package: package-deps package-prepare package-deb package-rpm

package-deps:
	# Installing packaging dependencies...
	gem install rake fpm --no-ri --no-rdoc

package-prepare:
	chmod 755 packaging/root/usr/share/gitlab-runner/
	chmod 755 packaging/root/usr/share/gitlab-runner/*

package-deb: package-deps package-prepare
	# Building Debian compatible packages...
	make package-deb-fpm ARCH=amd64 PACKAGE_ARCH=amd64
	make package-deb-fpm ARCH=386 PACKAGE_ARCH=i386
	make package-deb-fpm ARCH=arm PACKAGE_ARCH=armel
	make package-deb-fpm ARCH=arm PACKAGE_ARCH=armhf

package-rpm: package-deps package-prepare
	# Building RedHat compatible packages...
	make package-rpm-fpm ARCH=amd64 PACKAGE_ARCH=amd64
	make package-rpm-fpm ARCH=386 PACKAGE_ARCH=i686
	make package-rpm-fpm ARCH=arm PACKAGE_ARCH=arm
	make package-rpm-fpm ARCH=arm PACKAGE_ARCH=armhf

package-deb-fpm:
	@mkdir -p out/deb/
	fpm -s dir -t deb -n $(PACKAGE_NAME) -v $(VERSION) \
		-p out/deb/$(PACKAGE_NAME)_$(PACKAGE_ARCH).deb \
		--deb-priority optional --category admin \
		--force \
		--deb-compression bzip2 \
		--after-install packaging/scripts/postinst.deb \
		--before-remove packaging/scripts/prerm.deb \
		--url https://gitlab.com/gitlab-org/gitlab-runner \
		--description "GitLab Runner" \
		-m "GitLab Inc. <support@gitlab.com>" \
		--license "MIT" \
		--vendor "GitLab Inc." \
		--conflicts $(PACKAGE_NAME)-beta \
		--conflicts gitlab-ci-multi-runner \
		--conflicts gitlab-ci-multi-runner-beta \
		--provides gitlab-ci-multi-runner \
		--replaces gitlab-ci-multi-runner \
		--depends ca-certificates \
		--depends git \
		--depends curl \
		--depends tar \
		--deb-suggests docker-engine \
		-a $(PACKAGE_ARCH) \
		packaging/root/=/ \
		out/binaries/$(NAME)-linux-$(ARCH)=/usr/bin/gitlab-runner

package-rpm-fpm:
	@mkdir -p out/rpm/
	fpm -s dir -t rpm -n $(PACKAGE_NAME) -v $(VERSION) \
		-p out/rpm/$(PACKAGE_NAME)_$(PACKAGE_ARCH).rpm \
		--rpm-compression bzip2 --rpm-os linux \
		--force \
		--after-install packaging/scripts/postinst.rpm \
		--before-remove packaging/scripts/prerm.rpm \
		--url https://gitlab.com/gitlab-org/gitlab-runner \
		--description "GitLab Runner" \
		-m "GitLab Inc. <support@gitlab.com>" \
		--license "MIT" \
		--vendor "GitLab Inc." \
		--conflicts $(PACKAGE_NAME)-beta \
		--conflicts gitlab-ci-multi-runner \
		--conflicts gitlab-ci-multi-runner-beta \
		--provides gitlab-ci-multi-runner \
		--replaces gitlab-ci-multi-runner \
		--depends git \
		--depends curl \
		--depends tar \
		-a $(PACKAGE_ARCH) \
		packaging/root/=/ \
		out/binaries/$(NAME)-linux-$(ARCH)=/usr/bin/gitlab-runner

packagecloud: packagecloud-deps packagecloud-deb packagecloud-rpm

packagecloud-deps:
	# Installing packagecloud dependencies...
	gem install package_cloud --version "~> 0.3.0" --no-ri --no-rdoc

packagecloud-deb:
	# Sending Debian compatible packages...
	-for DIST in $(DEB_PLATFORMS); do \
		package_cloud push --url $(PACKAGE_CLOUD_URL) $(PACKAGE_CLOUD)/$$DIST out/deb/*.deb; \
	done

packagecloud-rpm:
	# Sending RedHat compatible packages...
	-for DIST in $(RPM_PLATFORMS); do \
		package_cloud push --url $(PACKAGE_CLOUD_URL) $(PACKAGE_CLOUD)/$$DIST out/rpm/*.rpm; \
	done

packagecloud-yank:
ifneq ($(YANK),)
	# Removing $(YANK) from packagecloud...
	-for DIST in $(DEB_PLATFORMS); do \
		for ARCH in $(DEB_ARCHS); do \
			package_cloud yank --url $(PACKAGE_CLOUD_URL) $(PACKAGE_CLOUD)/$$DIST $(PACKAGE_NAME)_$(YANK)_$$ARCH.deb & \
		done; \
	done; \
	for DIST in $(RPM_PLATFORMS); do \
		for ARCH in $(RPM_ARCHS); do \
			package_cloud yank --url $(PACKAGE_CLOUD_URL) $(PACKAGE_CLOUD)/$$DIST $(PACKAGE_NAME)-$(YANK)-1.$$ARCH.rpm & \
		done; \
	done; \
	wait
else
	# No version specified in YANK
	@exit 1
endif

s3-upload:
	export ARTIFACTS_DEST=artifacts; curl -sL https://raw.githubusercontent.com/travis-ci/artifacts/master/install | bash
	./artifacts upload \
		--permissions public-read \
		--working-dir out \
		--target-paths "$(S3_UPLOAD_PATH)/" \
		--max-size $(shell du -bs out/ | cut -f1) \
		$(shell cd out/; find . -type f)
	@echo "\n\033[1m==> Download index file: \033[36mhttps://$$ARTIFACTS_S3_BUCKET.s3.amazonaws.com/$$S3_UPLOAD_PATH/index.html\033[0m\n"

release_packagecloud:
	# Releasing to https://packages.gitlab.com/runner/
	@./ci/release_packagecloud "$$CI_JOB_NAME"

release_s3: prepare_windows_zip prepare_zoneinfo prepare_index
	# Releasing to S3
	@./ci/release_s3

prepare_windows_zip:
	# Prepare ZIP archives for Windows *.exe files
	@cd out/binaries/; \
	for file in $$(find * -type f -name "*windows*.exe" | sed 's/.exe$$//'); do \
		zip $${file}.zip $${file}.exe; \
	done

prepare_zoneinfo:
	# preparing the zoneinfo file
	@cp $$GOROOT/lib/time/zoneinfo.zip out/

prepare_index:
	# Preparing index file
	@./ci/prepare_index

release_docker_images:
	# Releasing Docker images
	@./ci/release_docker_images

check-tags-in-changelog:
	# Looking for tags in CHANGELOG
	@git status | grep "On branch master" 2>&1 >/dev/null || echo "Check should be done on master branch only. Skipping."
	@for tag in $$(git tag | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$$" | sed 's|v||' | sort -g); do \
		state="MISSING"; \
		grep "^v $$tag" CHANGELOG.md 2>&1 >/dev/null; \
		[ "$$?" -eq 1 ] || state="OK"; \
		echo "$$tag:   \t $$state"; \
	done

development_setup:
	test -d tmp/gitlab-test || git clone https://gitlab.com/gitlab-org/gitlab-test.git tmp/gitlab-test
	if prlctl --version ; then $(MAKE) -C tests/ubuntu parallels ; fi
	if vboxmanage --version ; then $(MAKE) -C tests/ubuntu virtualbox ; fi

# local GOPATH
$(GOPATH_SETUP): $(PKG_BUILD_DIR)
	mkdir -p $(GOPATH_BIN)
	touch $@

$(PKG_BUILD_DIR):
	mkdir -p $(@D)
	ln -s ../../../.. $@

# development tools
$(GOX): $(GOPATH_SETUP)
	go get github.com/mitchellh/gox

$(GOBINDATA): $(GOPATH_SETUP)
	go get github.com/jteeuwen/go-bindata/...

$(MOCKERY): $(GOPATH_SETUP)
	go get github.com/vektra/mockery/.../

clean:
	-$(RM) -rf $(LOCAL_GOPATH)
	-$(RM) executors/docker/bindata.go
	-$(RM) -rf $(TARGET_DIR)
