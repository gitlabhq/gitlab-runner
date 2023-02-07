NAME ?= gitlab-runner
APP_NAME ?= $(NAME)
export PACKAGE_NAME ?= $(NAME)
export VERSION := $(shell ./ci/version)
REVISION := $(shell git rev-parse --short=8 HEAD || echo unknown)
BRANCH := $(shell git show-ref | grep "$(REVISION)" | grep -v HEAD | awk '{print $$2}' | sed 's|refs/remotes/origin/||' | sed 's|refs/heads/||' | sort | head -n 1)
BUILT := $(shell date -u +%Y-%m-%dT%H:%M:%S%z)
export TESTFLAGS ?= -cover

LATEST_STABLE_TAG := $(shell git -c versionsort.prereleaseSuffix="-rc" -c versionsort.prereleaseSuffix="-RC" tag -l "v*.*.*" | sort -rV | awk '!/rc/' | head -n 1)
export IS_LATEST :=
ifeq ($(shell git describe --exact-match --match $(LATEST_STABLE_TAG) >/dev/null 2>&1; echo $$?), 0)
export IS_LATEST := true
endif

PACKAGE_CLOUD ?= ayufan/gitlab-ci-multi-runner
PACKAGE_CLOUD_URL ?= https://packagecloud.io/
BUILD_ARCHS ?= -arch '386' -arch 'arm' -arch 'amd64' -arch 'arm64' -arch 's390x' -arch 'ppc64le'
BUILD_PLATFORMS ?= -osarch 'darwin/amd64' -osarch 'darwin/arm64' -os 'linux' -os 'freebsd' -os 'windows' ${BUILD_ARCHS}
S3_UPLOAD_PATH ?= main

DEB_ARCHS ?= amd64 i386 armel armhf arm64 aarch64 s390x ppc64le
RPM_ARCHS ?= x86_64 i686 arm armhf arm64 aarch64 s390x ppc64le

PKG = gitlab.com/gitlab-org/$(PACKAGE_NAME)
COMMON_PACKAGE_NAMESPACE = $(PKG)/common

BUILD_DIR := $(CURDIR)
TARGET_DIR := $(BUILD_DIR)/out

export MAIN_PACKAGE ?= gitlab.com/gitlab-org/gitlab-runner

GO_LDFLAGS ?= -X $(COMMON_PACKAGE_NAMESPACE).NAME=$(APP_NAME) -X $(COMMON_PACKAGE_NAMESPACE).VERSION=$(VERSION) \
              -X $(COMMON_PACKAGE_NAMESPACE).REVISION=$(REVISION) -X $(COMMON_PACKAGE_NAMESPACE).BUILT=$(BUILT) \
              -X $(COMMON_PACKAGE_NAMESPACE).BRANCH=$(BRANCH) \
              -w
GO_FILES ?= $(shell find . -name '*.go')
export CGO_ENABLED ?= 0


# Development Tools
GOCOVER_COBERTURA = gocover-cobertura

GOX = gox

MOCKERY_VERSION ?= 2.14.0
MOCKERY = mockery

GOLANGLINT_VERSION ?= v1.46.2
GOLANGLINT ?= .tmp/golangci-lint$(GOLANGLINT_VERSION)
GOLANGLINT_GOARGS ?= .tmp/goargs.so

DEVELOPMENT_TOOLS = $(GOX) $(MOCKERY)

RELEASE_INDEX_GEN_VERSION ?= latest
RELEASE_INDEX_GENERATOR ?= .tmp/release-index-gen-$(RELEASE_INDEX_GEN_VERSION)
GITLAB_CHANGELOG_VERSION ?= latest
GITLAB_CHANGELOG = .tmp/gitlab-changelog-$(GITLAB_CHANGELOG_VERSION)

.PHONY: all
all: deps runner-and-helper-bin

include Makefile.runner_helper.mk
include Makefile.build.mk
include Makefile.package.mk

.PHONY: help
help:
	# Commands:
	# make all => install deps and build Runner binaries and Helper images
	# make version - show information about current version
	#
	# Development commands:
	# make development_setup - setup needed environment for tests
	# make runner-bin-host - build executable for your arch and OS
	# make runner-and-helper-bin-host - build executable for your arch and OS, including docker dependencies
	# make runner-and-helper-bin-linux - build executable for all supported architectures for linux OS, including docker dependencies
	# make runner-and-helper-bin - build executable for all supported platforms, including docker dependencies
	# make runner-and-helper-docker-host - build Alpine and Ubuntu Docker images with the runner executable and helper
	# make helper-dockerarchive - build Runner Helper docker dependencies for all supported platforms
	# make helper-dockerarchive-host - build Runner Helper docker dependencies for your oarch and OS
	#
	# Testing commands:
	# make test - run project tests
	# make lint - run code quality analysis
	# make lint-docs - run documentation linting
	#
	# Deployment commands:
	# make deps - install all dependencies
	# make runner-bin - build project for all supported platforms
	# make package - package project using FPM
	# make packagecloud - send all packages to packagecloud
	# make packagecloud-yank - remove specific version from packagecloud

.PHONY: version
version:
	@echo Current version: $(VERSION)
	@echo Current revision: $(REVISION)
	@echo Current branch: $(BRANCH)
	@echo Build platforms: $(BUILD_PLATFORMS)
	@echo DEB platforms: $(DEB_PLATFORMS)
	@echo RPM platforms: $(RPM_PLATFORMS)
	@echo IS_LATEST: $(IS_LATEST)

.PHONY: deps
deps: $(DEVELOPMENT_TOOLS)

.PHONY: check_test_directives
check_test_directives:
	@scripts/check_test_directives

.PHONY: lint
lint: OUT_FORMAT ?= colored-line-number
lint: LINT_FLAGS ?=
lint: $(GOLANGLINT)
	@$(MAKE) check_test_directives >/dev/stderr
	@$(GOLANGLINT) run ./... --out-format $(OUT_FORMAT) $(LINT_FLAGS)

.PHONY: lint-docs
lint-docs:
	@scripts/lint-docs

check_race_conditions:
	@./scripts/check_race_conditions

.PHONY: test
test: helper-dockerarchive-host development_setup simple-test

simple-test: TEST_PKG ?= $(shell go list ./...)
simple-test:
	# use env -i to clear parent environment variables for go test
	./scripts/go_test_no_env $(TEST_PKG) $(TESTFLAGS) -ldflags "$(GO_LDFLAGS)"

git1.8-test: export TEST_PKG = gitlab.com/gitlab-org/gitlab-runner/executors/shell gitlab.com/gitlab-org/gitlab-runner/shells
git1.8-test:
	$(MAKE) simple-test TESTFLAGS='-cover -tags=integration'
	$(MAKE) simple-test

cobertura_report: $(GOCOVER_COBERTURA)
	mkdir -p out/cobertura
	$(GOCOVER_COBERTURA) < out/coverage/coverprofile.regular.source.txt > out/cobertura/cobertura-coverage-raw.xml
	@ # NOTE: Remove package paths.
	@ # See https://gitlab.com/gitlab-org/gitlab/-/issues/217664
	sed 's;filename=\"gitlab.com/gitlab-org/gitlab-runner/;filename=\";g' out/cobertura/cobertura-coverage-raw.xml > \
	  out/cobertura/cobertura-coverage.xml

export_test_env:
	@echo "export GO_LDFLAGS='$(GO_LDFLAGS)'"
	@echo "export MAIN_PACKAGE='$(MAIN_PACKAGE)'"

parallel_test_prepare:
	# Preparing test commands
	@./scripts/go_test_with_coverage_report prepare

parallel_test_execute: export GO_LDFLAGS ?= "$(GO_LDFLAGS)"
parallel_test_execute: pull_images_for_tests
	# Executing tests
	@./scripts/go_test_with_coverage_report execute

parallel_test_coverage_report:
	# Preparing coverage report
	@./scripts/go_test_with_coverage_report coverage

parallel_test_junit_report:
	# Preparing jUnit test report
	@./scripts/go_test_with_coverage_report junit

check_windows_test_ignore_list:
	# Checking for orphaned test names in ignore file
	@./scripts/check_windows_test_ignore_list

pull_images_for_tests:
	# Pulling images required for some tests
	@go run ./scripts/pull-images-for-tests/main.go

dockerfiles:
	$(MAKE) -C dockerfiles all

.PHONY: mocks
mocks: $(MOCKERY)
	rm -rf ./helpers/service/mocks
	find . -type f -name 'mock_*' -delete
	go generate ./...

check_mocks: mocks
	# Checking the differences
	@git --no-pager diff --compact-summary --exit-code -- ./helpers/service/mocks \
		$(shell git ls-files | grep 'mock_') && \
		!(git ls-files -o | grep 'mock_') && \
		echo "Mocks up-to-date!"

test-docker:
	$(MAKE) test-docker-image IMAGE=centos:7 TYPE=rpm
	$(MAKE) test-docker-image IMAGE=debian:wheezy TYPE=deb
	$(MAKE) test-docker-image IMAGE=debian:jessie TYPE=deb
	$(MAKE) test-docker-image IMAGE=ubuntu-upstart:precise TYPE=deb
	$(MAKE) test-docker-image IMAGE=ubuntu-upstart:trusty TYPE=deb
	$(MAKE) test-docker-image IMAGE=ubuntu-upstart:utopic TYPE=deb

test-docker-image:
	tests/test_installation.sh $(IMAGE) out/$(TYPE)/$(PACKAGE_NAME)_amd64.$(TYPE)
	tests/test_installation.sh $(IMAGE) out/$(TYPE)/$(PACKAGE_NAME)_amd64.$(TYPE) Y

build-and-deploy: ARCH ?= amd64
build-and-deploy:
	$(MAKE) runner-and-helper-bin BUILD_PLATFORMS="-osarch=linux/$(ARCH)"
	$(MAKE) package-deb-arch ARCH=$(ARCH) PACKAGE_ARCH=$(ARCH)
	@[ -z "$(SERVER)" ] && echo "SERVER variable not specified!" && exit 1
	scp out/deb/$(PACKAGE_NAME)_$(ARCH).deb $(SERVER):
	ssh $(SERVER) dpkg -i $(PACKAGE_NAME)_$(ARCH).deb

build-and-deploy-binary: ARCH ?= amd64
build-and-deploy-binary:
	$(MAKE) runner-bin BUILD_PLATFORMS="-osarch=linux/$(ARCH)"
	@[ -z "$(SERVER)" ] && echo "SERVER variable not specified!" && exit 1
	scp out/binaries/$(PACKAGE_NAME)-linux-$(ARCH) $(SERVER):/usr/bin/gitlab-runner

.PHONY: packagecloud
packagecloud: packagecloud-deps packagecloud-deb packagecloud-rpm

packagecloud-deps:
	# Installing packagecloud dependencies...
	gem install package_cloud --version "~> 0.3.0" --no-document

packagecloud-deb:
	# Sending Debian compatible packages...
	@-./ci/push_packagecloud $(PACKAGE_CLOUD_URL) $(PACKAGE_CLOUD) deb "$(CI_JOB_NAME)"

packagecloud-rpm:
	# Sending RedHat compatible packages...
	@-./ci/push_packagecloud $(PACKAGE_CLOUD_URL) $(PACKAGE_CLOUD) rpm "$(CI_JOB_NAME)"

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

out/binaries/gitlab-runner-windows-%.zip: out/binaries/gitlab-runner-windows-%.exe
	zip --junk-paths $@ $<
	cd out/ && zip -r ../$@ helper-images

prepare_windows_zip: out/binaries/gitlab-runner-windows-386.zip out/binaries/gitlab-runner-windows-amd64.zip

prepare_zoneinfo:
	# preparing the zoneinfo file
	@cp $(shell go env GOROOT)/lib/time/zoneinfo.zip out/

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
	# Releasing GitLab Runner images
	@./ci/release_docker_images

release_helper_docker_images:
	# Releasing GitLab Runner Helper images
	@./ci/release_helper_docker_images

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
	@git status | grep "On branch main" 2>&1 >/dev/null || echo "Check should be done on main branch only. Skipping."
	@for tag in $$(git tag | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$$" | sed 's|v||' | sort -g); do \
		state="MISSING"; \
		grep "^v $$tag" CHANGELOG.md 2>&1 >/dev/null; \
		[ "$$?" -eq 1 ] || state="OK"; \
		echo "$$tag:   \t $$state"; \
	done

update_feature_flags_docs:
	go run ./scripts/update-feature-flags-docs/main.go

development_setup:
	test -d tmp/gitlab-test || git clone https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test tmp/gitlab-test
	if prlctl --version ; then $(MAKE) -C tests/ubuntu parallels ; fi
	if vboxmanage --version ; then $(MAKE) -C tests/ubuntu virtualbox ; fi

check_modules:
	# check go.sum
	@git checkout HEAD -- go.sum
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-before
	@go mod tidy
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-after
	@diff -U0 /tmp/gosum-$${CI_JOB_ID}-before /tmp/gosum-$${CI_JOB_ID}-after


# development tools
$(GOCOVER_COBERTURA):
	go install github.com/boumenot/gocover-cobertura@v1.2.0

$(GOX):
	go install github.com/mitchellh/gox@v1.0.1

$(GOLANGLINT): TOOL_BUILD_DIR := .tmp/build/golangci-lint
$(GOLANGLINT): $(GOLANGLINT_GOARGS)
	rm -rf $(TOOL_BUILD_DIR)
	git clone https://github.com/golangci/golangci-lint.git --no-tags --depth 1 -b "$(GOLANGLINT_VERSION)" $(TOOL_BUILD_DIR) && \
	cd $(TOOL_BUILD_DIR) && \
	export COMMIT=$(shell git rev-parse --short HEAD) && \
	export DATE=$(shell date -u '+%FT%TZ') && \
	CGO_ENABLED=1 go build --trimpath -o $(BUILD_DIR)/$(GOLANGLINT) \
		-ldflags "-s -w -X main.version=$(GOLANGLINT_VERSION) -X main.commit=$${COMMIT} -X main.date=$${DATE}" \
		./cmd/golangci-lint/ && \
	cd $(BUILD_DIR) && \
	rm -rf $(TOOL_BUILD_DIR) && \
	$(GOLANGLINT) --version

$(GOLANGLINT_GOARGS): TOOL_BUILD_DIR := .tmp/build/goargs
$(GOLANGLINT_GOARGS):
	rm -rf $(TOOL_BUILD_DIR)
	git clone https://gitlab.com/gitlab-org/language-tools/go/linters/goargs.git --no-tags --depth 1 $(TOOL_BUILD_DIR)
	cd $(TOOL_BUILD_DIR) && \
	CGO_ENABLED=1 go build --trimpath --buildmode=plugin -o $(BUILD_DIR)/$(GOLANGLINT_GOARGS) plugin/analyzer.go
	rm -rf $(TOOL_BUILD_DIR)

.PHONY: $(MOCKERY)
$(MOCKERY):
	go install github.com/vektra/mockery/v2@v$(MOCKERY_VERSION)

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

.PHONY: clean
clean:
	-$(RM) -rf $(TARGET_DIR)
	-$(RM) -rf tmp/gitlab-test

print_ldflags:
	@echo $(GO_LDFLAGS)
