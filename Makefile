NAME ?= gitlab-runner
export PACKAGE_NAME ?= $(NAME)
export VERSION := $(shell ./ci/version)
REVISION := $(shell git rev-parse --short=8 HEAD || echo unknown)
BRANCH := $(shell git show-ref | grep "$(REVISION)" | grep -v HEAD | awk '{print $$2}' | sed 's|refs/remotes/origin/||' | sed 's|refs/heads/||' | sort | head -n 1)
BUILT := $(shell date -u +%Y-%m-%dT%H:%M:%S%z)
export TESTFLAGS ?= -cover

LATEST_STABLE_TAG := $(shell git -c versionsort.prereleaseSuffix="-rc" -c versionsort.prereleaseSuffix="-RC" tag -l "v*.*.*" --sort=-v:refname | awk '!/rc/' | head -n 1)
export IS_LATEST :=
ifeq ($(shell git describe --exact-match --match $(LATEST_STABLE_TAG) >/dev/null 2>&1; echo $$?), 0)
export IS_LATEST := true
endif

PACKAGE_CLOUD ?= ayufan/gitlab-ci-multi-runner
PACKAGE_CLOUD_URL ?= https://packagecloud.io/
BUILD_PLATFORMS ?= -os '!netbsd' -os '!openbsd' -arch '!mips' -arch '!mips64' -arch '!mipsle' -arch '!mips64le' -arch '!s390x'
S3_UPLOAD_PATH ?= master

# Keep in sync with docs/install/linux-repository.md
DEB_PLATFORMS ?= debian/jessie debian/stretch debian/buster \
    ubuntu/xenial ubuntu/bionic \
    raspbian/jessie raspbian/stretch raspbian/buster \
    linuxmint/sarah linuxmint/serena linuxmint/sonya
DEB_ARCHS ?= amd64 i386 armel armhf arm64 aarch64
RPM_PLATFORMS ?= el/6 el/7 \
    ol/6 ol/7 \
    fedora/30
RPM_ARCHS ?= x86_64 i686 arm armhf arm64 aarch64

PKG = gitlab.com/gitlab-org/$(PACKAGE_NAME)
COMMON_PACKAGE_NAMESPACE=$(PKG)/common

BUILD_DIR := $(CURDIR)
TARGET_DIR := $(BUILD_DIR)/out

# Packages in vendor/ are included in ./...
# https://github.com/golang/go/issues/11659
export OUR_PACKAGES ?= $(subst _$(BUILD_DIR),$(PKG),$(shell go list ./... | grep -v '/vendor/'))

GO_LDFLAGS ?= -X $(COMMON_PACKAGE_NAMESPACE).NAME=$(PACKAGE_NAME) -X $(COMMON_PACKAGE_NAMESPACE).VERSION=$(VERSION) \
              -X $(COMMON_PACKAGE_NAMESPACE).REVISION=$(REVISION) -X $(COMMON_PACKAGE_NAMESPACE).BUILT=$(BUILT) \
              -X $(COMMON_PACKAGE_NAMESPACE).BRANCH=$(BRANCH) \
              -s -w
GO_FILES ?= $(shell find . -name '*.go')
export CGO_ENABLED ?= 0


# Development Tools
GOX = gox

MOCKERY_VERSION ?= 1.1.0
MOCKERY ?= .tmp/mockery-$(MOCKERY_VERSION)

DEVELOPMENT_TOOLS = $(GOX) $(MOCKERY)

RELEASE_INDEX_GEN_VERSION ?= latest
RELEASE_INDEX_GENERATOR ?= .tmp/release-index-gen-$(RELEASE_INDEX_GEN_VERSION)
GITLAB_CHANGELOG_VERSION ?= latest
GITLAB_CHANGELOG = .tmp/gitlab-changelog-$(GITLAB_CHANGELOG_VERSION)

.PHONY: clean version mocks

all: deps helper-docker build_all

include Makefile.runner_helper.mk
include Makefile.build.mk
include Makefile.package.mk

help:
	# Commands:
	# make all => deps build
	# make version - show information about current version
	#
	# Development commands:
	# make development_setup - setup needed environment for tests
	# make build_simple - build executable for your arch and OS
	# make build_current - build executable for your arch and OS, including docker dependencies
	# make helper-docker - build docker dependencies
	#
	# Testing commands:
	# make test - run project tests
	# make lint - run code quality analysis
	# make lint-docs - run documentation linting
	#
	# Deployment commands:
	# make deps - install all dependencies
	# make build_all - build project for all supported OSes
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

lint: OUT_FORMAT ?= colored-line-number
lint: LINT_FLAGS ?=
lint:
	@golangci-lint run ./... --out-format $(OUT_FORMAT) $(LINT_FLAGS)

lint-docs:
	@scripts/lint-docs

check_race_conditions:
	@./scripts/check_race_conditions $(OUR_PACKAGES)

test: helper-docker development_setup simple-test

simple-test:
	go test $(OUR_PACKAGES) $(TESTFLAGS)

parallel_test_prepare:
	# Preparing test commands
	@./scripts/go_test_with_coverage_report prepare

parallel_test_execute: pull_images_for_tests
	# executing tests
	@./scripts/go_test_with_coverage_report execute

parallel_test_coverage_report:
	# Preparing coverage report
	@./scripts/go_test_with_coverage_report coverage

parallel_test_junit_report:
	# Preparing jUnit test report
	@./scripts/go_test_with_coverage_report junit

pull_images_for_tests:
	# Pulling images required for some tests
	@go run ./scripts/pull-images-for-tests/main.go

dockerfiles:
	$(MAKE) -C dockerfiles all

mocks: $(MOCKERY)
	rm -rf ./helpers/service/mocks
	find . -type f ! -path '*vendor/*' -name 'mock_*' -delete
	$(MOCKERY) -dir=./vendor/github.com/ayufan/golang-kardianos-service -output=./helpers/service/mocks -name='(Interface)'
	$(MOCKERY) -dir=./network -name='requester' -inpkg
	$(MOCKERY) -dir=./helpers -all -inpkg
	$(MOCKERY) -dir=./executors/docker -all -inpkg
	$(MOCKERY) -dir=./executors/kubernetes -all -inpkg
	$(MOCKERY) -dir=./executors/custom -all -inpkg
	$(MOCKERY) -dir=./cache -all -inpkg
	$(MOCKERY) -dir=./common -all -inpkg
	$(MOCKERY) -dir=./log -all -inpkg
	$(MOCKERY) -dir=./referees -all -inpkg
	$(MOCKERY) -dir=./session -all -inpkg
	$(MOCKERY) -dir=./shells -all -inpkg

check_mocks:
	# Checking if mocks are up-to-date
	@$(MAKE) mocks
	# Checking the differences
	@git --no-pager diff --compact-summary --exit-code -- ./helpers/service/mocks \
		$(shell git ls-files | grep 'mock_' | grep -v 'vendor/') && \
		echo "Mocks up-to-date!"

test-docker:
	$(MAKE) test-docker-image IMAGE=centos:6 TYPE=rpm
	$(MAKE) test-docker-image IMAGE=centos:7 TYPE=rpm
	$(MAKE) test-docker-image IMAGE=debian:wheezy TYPE=deb
	$(MAKE) test-docker-image IMAGE=debian:jessie TYPE=deb
	$(MAKE) test-docker-image IMAGE=ubuntu-upstart:precise TYPE=deb
	$(MAKE) test-docker-image IMAGE=ubuntu-upstart:trusty TYPE=deb
	$(MAKE) test-docker-image IMAGE=ubuntu-upstart:utopic TYPE=deb

test-docker-image:
	tests/test_installation.sh $(IMAGE) out/$(TYPE)/$(PACKAGE_NAME)_amd64.$(TYPE)
	tests/test_installation.sh $(IMAGE) out/$(TYPE)/$(PACKAGE_NAME)_amd64.$(TYPE) Y

build-and-deploy:
	$(MAKE) build_all BUILD_PLATFORMS="-os=linux -arch=amd64"
	$(MAKE) package-deb-fpm ARCH=amd64 PACKAGE_ARCH=amd64
	scp out/deb/$(PACKAGE_NAME)_amd64.deb $(SERVER):
	ssh $(SERVER) dpkg -i $(PACKAGE_NAME)_amd64.deb

build-and-deploy-binary:
	$(MAKE) build_all BUILD_PLATFORMS="-os=linux -arch=amd64"
	scp out/binaries/$(PACKAGE_NAME)-linux-amd64 $(SERVER):/usr/bin/gitlab-runner

packagecloud: packagecloud-deps packagecloud-deb packagecloud-rpm

packagecloud-deps:
	# Installing packagecloud dependencies...
	gem install package_cloud --version "~> 0.3.0" --no-document

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

release_s3: copy_helper_binaries prepare_windows_zip prepare_zoneinfo prepare_index
	# Releasing to S3
	@./ci/release_s3

out/binaries/gitlab-runner-windows-%.zip: out/binaries/gitlab-runner-windows-%.exe
	zip --junk-paths $@ $<
	cd out/ && zip -r ../$@ helper-images

prepare_windows_zip: out/binaries/gitlab-runner-windows-386.zip out/binaries/gitlab-runner-windows-amd64.zip

prepare_zoneinfo:
	# preparing the zoneinfo file
	@cp $$GOROOT/lib/time/zoneinfo.zip out/

copy_helper_binaries:
	# copying helper binaries
	@mkdir -p out/binaries/gitlab-runner-helper
	@cp dockerfiles/build/binaries/gitlab-runner-helper* out/binaries/gitlab-runner-helper/

prepare_index: export CI_COMMIT_REF_NAME ?= $(BRANCH)
prepare_index: export CI_COMMIT_SHA ?= $(REVISION)
prepare_index: $(RELEASE_INDEX_GENERATOR)
	# Preparing index file
	@$(RELEASE_INDEX_GENERATOR) -working-directory out/ \
						      -project-version $(VERSION) \
						      -project-git-ref $(CI_COMMIT_REF_NAME) \
						      -project-git-revision $(CI_COMMIT_SHA) \
						      -project-name "GitLab Runner" \
						      -project-repo-url "https://gitlab.com/gitlab-org/gitlab-runner" \
						      -gpg-key-env GPG_KEY \
						      -gpg-password-env GPG_PASSPHRASE

release_docker_images:
	# Releasing Docker images
	@./ci/release_docker_images

generate_changelog: export CHANGELOG_RELEASE ?= $(VERSION)
generate_changelog: $(GITLAB_CHANGELOG)
	# Generating new changelog entries
	@$(GITLAB_CHANGELOG) -project-id 250833 \
		-release $(CHANGELOG_RELEASE) \
		-starting-point-matcher "v[0-9]*.[0-9]*.[0-9]*" \
		-config-file .gitlab/changelog.yml \
		-changelog-file CHANGELOG.md

check-tags-in-changelog:
	# Looking for tags in CHANGELOG
	@git status | grep "On branch master" 2>&1 >/dev/null || echo "Check should be done on master branch only. Skipping."
	@for tag in $$(git tag | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$$" | sed 's|v||' | sort -g); do \
		state="MISSING"; \
		grep "^v $$tag" CHANGELOG.md 2>&1 >/dev/null; \
		[ "$$?" -eq 1 ] || state="OK"; \
		echo "$$tag:   \t $$state"; \
	done

update_feature_flags_docs:
	go run ./scripts/update-feature-flags-docs/main.go

development_setup:
	test -d tmp/gitlab-test || git clone https://gitlab.com/gitlab-org/ci-cd/tests/gitlab-test.git tmp/gitlab-test
	if prlctl --version ; then $(MAKE) -C tests/ubuntu parallels ; fi
	if vboxmanage --version ; then $(MAKE) -C tests/ubuntu virtualbox ; fi

check_modules:
	# Check if there is any difference in vendor/
	@git status -sb vendor/ > /tmp/vendor-$${CI_JOB_ID}-before
	@go mod vendor
	@git status -sb vendor/ > /tmp/vendor-$${CI_JOB_ID}-after
	@diff -U0 /tmp/vendor-$${CI_JOB_ID}-before /tmp/vendor-$${CI_JOB_ID}-after

	# check go.sum
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-before
	@go mod tidy
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-after
	@diff -U0 /tmp/gosum-$${CI_JOB_ID}-before /tmp/gosum-$${CI_JOB_ID}-after

# development tools
$(GOX):
	go get github.com/mitchellh/gox

$(MOCKERY): OS_TYPE ?= $(shell uname -s)
$(MOCKERY): DOWNLOAD_URL = "https://github.com/vektra/mockery/releases/download/v$(MOCKERY_VERSION)/mockery_$(MOCKERY_VERSION)_$(OS_TYPE)_x86_64.tar.gz"
$(MOCKERY):
	# Installing $(DOWNLOAD_URL) as $(MOCKERY)
	@mkdir -p $(shell dirname $(MOCKERY))
	@curl -sL "$(DOWNLOAD_URL)" | tar xz -O mockery > $(MOCKERY)
	@chmod +x "$(MOCKERY)"

$(RELEASE_INDEX_GENERATOR): OS_TYPE ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
$(RELEASE_INDEX_GENERATOR): DOWNLOAD_URL = "https://storage.googleapis.com/gitlab-runner-tools/release-index-generator/$(RELEASE_INDEX_GEN_VERSION)/release-index-gen-$(OS_TYPE)-amd64"
$(RELEASE_INDEX_GENERATOR):
	# Installing $(DOWNLOAD_URL) as $(RELEASE_INDEX_GENERATOR)
	@mkdir -p $(shell dirname $(RELEASE_INDEX_GENERATOR))
	@curl -sL "$(DOWNLOAD_URL)" -o "$(RELEASE_INDEX_GENERATOR)"
	@chmod +x "$(RELEASE_INDEX_GENERATOR)"

$(GITLAB_CHANGELOG): OS_TYPE ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
$(GITLAB_CHANGELOG): DOWNLOAD_URL = "https://storage.googleapis.com/gitlab-runner-tools/gitlab-changelog/$(GITLAB_CHANGELOG_VERSION)/gitlab-changelog-$(OS_TYPE)-amd64"
$(GITLAB_CHANGELOG):
	# Installing $(DOWNLOAD_URL) as $(GITLAB_CHANGELOG)
	@mkdir -p $(shell dirname $(GITLAB_CHANGELOG))
	@curl -sL "$(DOWNLOAD_URL)" -o "$(GITLAB_CHANGELOG)"
	@chmod +x "$(GITLAB_CHANGELOG)"

clean:
	-$(RM) -rf $(TARGET_DIR)
