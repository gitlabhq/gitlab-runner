NAME ?= gitlab-runner
APP_NAME ?= $(NAME)
export PACKAGE_NAME ?= $(NAME)
export VERSION := $(shell ./ci/version)
REVISION := $(shell git rev-parse --short=8 HEAD || echo unknown)
BRANCH := $(shell git show-ref | grep "$(REVISION)" | grep -v HEAD | awk '{print $$2}' | sed 's|refs/remotes/origin/||' | sed 's|refs/heads/||' | sort | head -n 1)
export TESTFLAGS ?= -cover

LATEST_STABLE_TAG := $(shell git -c versionsort.prereleaseSuffix="-rc" -c versionsort.prereleaseSuffix="-RC" tag -l "v*.*.*" | sort -rV | awk '!/rc/' | head -n 1)
export IS_LATEST :=
ifeq ($(shell git describe --exact-match --match $(LATEST_STABLE_TAG) >/dev/null 2>&1; echo $$?), 0)
export IS_LATEST := true
endif

PACKAGE_CLOUD ?= runner/gitlab-runner
PACKAGE_CLOUD_URL ?= https://packages.gitlab.com
BUILD_ARCHS ?= -arch '386' -arch 'arm' -arch 'amd64' -arch 'arm64' -arch 's390x' -arch 'ppc64le' -arch 'riscv64' -arch 'loong64'
BUILD_PLATFORMS ?= -osarch 'darwin/amd64' -osarch 'darwin/arm64' -os 'linux' -os 'freebsd' -os 'windows' ${BUILD_ARCHS}
S3_UPLOAD_PATH ?= main

ifeq ($(shell mage >/dev/null 2>&1; echo $$?), 0)
DEB_ARCHS := $(shell mage package:archs deb)
RPM_ARCHS := $(shell mage package:archs rpm)
endif

PKG = gitlab.com/gitlab-org/$(PACKAGE_NAME)
COMMON_PACKAGE_NAMESPACE = $(PKG)/common

BUILD_DIR := $(CURDIR)
TARGET_DIR := $(BUILD_DIR)/out

export MAIN_PACKAGE ?= gitlab.com/gitlab-org/gitlab-runner

GO_LDFLAGS ?= -X $(COMMON_PACKAGE_NAMESPACE).NAME=$(APP_NAME) -X $(COMMON_PACKAGE_NAMESPACE).VERSION=$(VERSION) \
              -X $(COMMON_PACKAGE_NAMESPACE).BRANCH=$(BRANCH) \
              -w

GO_TEST_LDFLAGS ?= -X $(COMMON_PACKAGE_NAMESPACE).NAME=$(APP_NAME)

GO_FILES ?= $(shell find . -name '*.go')
export CGO_ENABLED ?= 0

local := $(PWD)/.tmp
localBin := $(local)/bin

export GOBIN=$(localBin)
export PATH := $(localBin):$(PATH)

# Development Tools
GOCOVER_COBERTURA = gocover-cobertura

MOCKERY_VERSION ?= 3.6.1
MOCKERY = mockery

PROTOC := $(localBin)/protoc
PROTOC_VERSION := 28.2

PROTOC_GEN_GO := protoc-gen-go
PROTOC_GEN_GO_VERSION := v1.36.10

PROTOC_GEN_GO_GRPC := protoc-gen-go-grpc
PROTOC_GEN_GO_GRPC_VERSION := v1.6.0

SPLITIC = splitic
MAGE = $(localBin)/mage

GOLANGLINT_VERSION ?= 2.7.2
GOLANGLINT ?= $(localBin)/golangci-lint
GOLANGLINT_GOARGS ?= $(localBin)/goargs.so

GENERATED_FILES_TOOLS = $(MOCKERY) $(PROTOC) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)
DEVELOPMENT_TOOLS = $(MOCKERY) $(MAGE)

RELEASE_INDEX_GEN_VERSION ?= latest
RELEASE_INDEX_GENERATOR ?= $(localBin)/release-index-gen-$(RELEASE_INDEX_GEN_VERSION)
GITLAB_CHANGELOG_VERSION ?= latest
GITLAB_CHANGELOG = $(localBin)/gitlab-changelog-$(GITLAB_CHANGELOG_VERSION)

.PHONY: all
all: deps runner-and-helper-bin

include Makefile.runner_helper.mk
include Makefile.build.mk

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
	# make tools - install all dev tools and dependency binaries for local development
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
	#
	# Local Docker support commands
	# make runner-bin-linux - build runner linux binary, on any host OS
	# make helper-bin-linux - build helper linux binary, on any host OS
	# make runner-local-image - build gitlab-runner:local docker image
	# make helper-local-image - build gitlab-runner-helper:local docker image
	# make runner-and-helper-local-image - same as make runner-local-image helper-local-image

.PHONY: version
version:
	@echo Current version: $(VERSION)
	@echo Current revision: $(REVISION)
	@echo Current branch: $(BRANCH)
	@echo Build platforms: $(BUILD_PLATFORMS)
	@echo DEB archs: $(DEB_ARCHS)
	@echo RPM archs: $(RPM_ARCHS)
	@echo IS_LATEST: $(IS_LATEST)

.tmp:
	mkdir -p .tmp

.PHONY: deps
deps: $(DEVELOPMENT_TOOLS)

.PHONY: format
format: $(GOLANGLINT)
	@$(GOLANGLINT) run --fix --output.text.path=stdout --output.text.colors=true ./...

.PHONY: lint
lint: OUT_FORMAT ?= --output.text.path=stdout --output.text.colors=true
lint: LINT_FLAGS ?=
lint: $(GOLANGLINT)
	@$(MAKE) check_test_directives >/dev/stderr
	@$(GOLANGLINT) run $(OUT_FORMAT) $(LINT_FLAGS) ./...

.PHONY: lint-docs
lint-docs:
	@scripts/lint-docs

.PHONY: lint-i18n-docs
lint-i18n-docs:
	@scripts/lint-i18n-docs

.PHONY: format-ci-yaml
format-ci-yaml:
	prettier --write ".gitlab/ci/*.{yaml,yml}"

.PHONY: lint-ci-yaml
lint-ci-yaml:
	prettier --check ".gitlab/ci/**/*.{yml,yaml}" --log-level warn

.PHONY: test
test: development_setup simple-test

.PHONY: test-compile
test-compile:
	go test -count=1 --tags=integration -run=nope ./...
	go test -count=1 --tags=integration,steps -run=nope ./...
	go test -count=1 --tags=integration,kubernetes -run=nope ./...
	go test -count=1 -run=nope ./...

simple-test: TEST_PKG ?= $(shell go list ./...)
simple-test:
	# use env -i to clear parent environment variables for go test
	go test $(TEST_PKG) $(TESTFLAGS) -ldflags "$(GO_LDFLAGS)"

mage-test:
	go test -ldflags "$(GO_LDFLAGS)" -v ./magefiles/...

cobertura_report: $(GOCOVER_COBERTURA) $(SPLITIC)
	mkdir -p out/cobertura
	mkdir -p out/coverage
	$(SPLITIC) cover-merge $(wildcard .splitic/cover_?.profile) > out/coverage/coverprofile.regular.source.txt
	$(SPLITIC) cover-merge $(wildcard .splitic/cover_windows_?.profile) > out/coverage/coverprofile_windows.regular.source.txt
	GOOS=linux $(GOCOVER_COBERTURA) < out/coverage/coverprofile.regular.source.txt > out/cobertura/cobertura-coverage-raw.xml
	GOOS=windows $(GOCOVER_COBERTURA) < out/coverage/coverprofile_windows.regular.source.txt > out/cobertura/cobertura-coverage-windows-raw.xml
	@ # NOTE: Remove package paths.
	@ # See https://gitlab.com/gitlab-org/gitlab/-/issues/217664
	sed 's;filename=\"gitlab.com/gitlab-org/gitlab-runner/;filename=\";g' out/cobertura/cobertura-coverage-raw.xml > \
	  out/cobertura/cobertura-coverage.xml
	sed 's;filename=\"gitlab.com/gitlab-org/gitlab-runner/;filename=\";g' out/cobertura/cobertura-coverage-windows-raw.xml > \
	  out/cobertura/cobertura-windows-coverage.xml

export_test_env:
	@echo "export GO_LDFLAGS='$(GO_LDFLAGS)'"
	@echo "export MAIN_PACKAGE='$(MAIN_PACKAGE)'"

dockerfiles:
	$(MAKE) -C dockerfiles all

.PHONY: generated_files
generated_files: $(GENERATED_FILES_TOOLS)
	rm -rf ./helpers/service/mocks
	find . -type f -name 'mock_*' -delete
	find . -type f -name '*.pb.go' -delete
	go generate -v -x ./...
	cd ./helpers/runner_wrapper/api && go generate -v -x ./...
	$(localBin)/$(MOCKERY)

check_generated_files: generated_files
	# Checking the differences
	@git --no-pager diff --compact-summary --exit-code -- ./helpers/service/mocks \
		$(shell git ls-files | grep -e "mock_" -e "\.pb\.go") && \
		!(git ls-files -o | grep -e "mock_" -e "\.pb\.go") && \
		echo "Generated files up-to-date!"

generate_magefiles:
	$(shell mage generate)

check_magefiles: generate_magefiles
	# Checking the differences
	@git --no-pager diff --compact-summary --exit-code -- ./magefiles \
		$(shell git ls-files | grep '^magefiles/') && \
		!(git ls-files -o | grep '^magefiles/') && \
		echo "Magefiles up-to-date!"

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

release_s3: prepare_windows_zip prepare_zoneinfo release_dir prepare_index
	# Releasing to S3
	@./ci/release_s3

release_dir:
	@./ci/release_dir

prepare_windows_zip: out/binaries/gitlab-runner-windows-386.zip out/binaries/gitlab-runner-windows-amd64.zip

out/binaries/gitlab-runner-windows-386.zip: out/binaries/gitlab-runner-windows-386.exe
	zip -j out/binaries/gitlab-runner-windows-386.zip out/binaries/gitlab-runner-windows-386.exe
	cd out && zip binaries/gitlab-runner-windows-386.zip helper-images/prebuilt-*.tar.xz

out/binaries/gitlab-runner-windows-amd64.zip: out/binaries/gitlab-runner-windows-amd64.exe
	zip -j out/binaries/gitlab-runner-windows-amd64.zip out/binaries/gitlab-runner-windows-amd64.exe
	cd out && zip binaries/gitlab-runner-windows-amd64.zip helper-images/prebuilt-*.tar.xz

prepare_zoneinfo:
	# preparing the zoneinfo file
	@cp $(shell go env GOROOT)/lib/time/zoneinfo.zip out/

prepare_index: export CI_COMMIT_REF_NAME ?= $(BRANCH)
prepare_index: export CI_COMMIT_SHA ?= $(REVISION)
prepare_index: $(RELEASE_INDEX_GENERATOR)
	# Preparing index file
	@$(RELEASE_INDEX_GENERATOR) -working-directory out/release \
								-project-version $(VERSION) \
								-project-git-ref $(CI_COMMIT_REF_NAME) \
								-project-git-revision $(CI_COMMIT_SHA) \
								-project-name "GitLab Runner" \
								-project-repo-url "https://gitlab.com/gitlab-org/gitlab-runner" \
								-gpg-key-env GPG_KEY \
								-gpg-password-env GPG_PASSPHRASE

run_go_script: export SCRIPT_NAME ?=
run_go_script: export DEFAULT_ARGS ?=
run_go_script: export ARGS ?=
run_go_script:
	@cd scripts && go run $(SCRIPT_NAME)/main.go \
		$(DEFAULT_ARGS) \
		$(ARGS)

sync_docker_images: export ARGS ?= --concurrency=3
sync_docker_images:
	@$(MAKE) \
		SCRIPT_NAME=sync-docker-images \
		DEFAULT_ARGS="--revision $(REVISION)" \
		ARGS="$(ARGS)" \
		run_go_script

check_test_directives:
	@$(MAKE) \
		SCRIPT_NAME=check-test-directives \
		ARGS="$(shell pwd)" \
		run_go_script

update_feature_flags_docs:
	@$(MAKE) \
		SCRIPT_NAME=update-feature-flags-docs \
		ARGS="$(shell pwd)" \
		run_go_script

packagecloud_releases: export ARGS ?=
packagecloud_releases:
	@$(MAKE) \
		SCRIPT_NAME=packagecloud-releases \
		ARGS="$(ARGS)" \
		run_go_script

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

development_setup:
	test -d tmp/gitlab-test || git clone https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test tmp/gitlab-test
	if prlctl --version ; then $(MAKE) -C tests/ubuntu parallels ; fi
	if vboxmanage --version ; then $(MAKE) -C tests/ubuntu virtualbox ; fi

check_modules:
	# check go.mod and go.sum
	@git checkout HEAD -- go.mod go.sum
	@git diff go.mod go.sum > /tmp/gomodsum-$${CI_JOB_ID}-before
	@go mod tidy
	@git diff go.mod go.sum > /tmp/gomodsum-$${CI_JOB_ID}-after
	@diff -U0 /tmp/gomodsum-$${CI_JOB_ID}-before /tmp/gomodsum-$${CI_JOB_ID}-after

	# check dependency resolution
	@go list -m all >/dev/null

	# check helpers/runner_wrapper/api/ go.sum
	@cd ./helpers/runner_wrapper/api/
	@git checkout HEAD -- go.mod go.sum
	@git diff go.mod go.sum > /tmp/gomodsum-$${CI_JOB_ID}-before
	@go mod tidy
	@git diff go.mod go.sum > /tmp/gomodsum-$${CI_JOB_ID}-after
	@diff -U0 /tmp/gomodsum-$${CI_JOB_ID}-before /tmp/gomodsum-$${CI_JOB_ID}-after

	# check dependency helpers/runner_wrapper/api/ resolution
	@go list -m all >/dev/null

# development tools
$(GOCOVER_COBERTURA):
	@go install github.com/boumenot/gocover-cobertura@v1.2.0

$(SPLITIC):
	@go install gitlab.com/gitlab-org/ci-cd/runner-tools/splitic@latest

.PHONY: mage
mage: $(MAGE)
	@:
$(MAGE): .tmp
	cd .tmp && \
	rm -rf mage && \
	git clone https://github.com/magefile/mage && \
	cd mage && \
	GOPATH=$(local) go run bootstrap.go
	# Remove the source code once binary built
	# Go intentionally makes module cache directories read-only to prevent accidental modifications
	GOPATH=$(local) go clean -modcache
	rm -rf .tmp/mage .tmp/pkg

ifneq ($(GOLANGLINT_VERSION),)
$(GOLANGLINT): CHECKOUT_REF := -b v"$(GOLANGLINT_VERSION)"
endif
$(GOLANGLINT): TOOL_BUILD_DIR := .tmp/build/golangci-lint
$(GOLANGLINT): $(GOLANGLINT_GOARGS)
$(GOLANGLINT):
	rm -rf $(TOOL_BUILD_DIR)
	git clone https://github.com/golangci/golangci-lint.git --no-tags --depth 1 $(CHECKOUT_REF) $(TOOL_BUILD_DIR)
	cd $(TOOL_BUILD_DIR) && \
	export COMMIT=$(shell git rev-parse --short HEAD) && \
	export DATE=$(shell date -u '+%FT%TZ') && \
	CGO_ENABLED=1 go build --trimpath -o $(GOLANGLINT) \
		-ldflags "-s -w -X main.version=v$(GOLANGLINT_VERSION) -X main.commit=$${COMMIT} -X main.date=$${DATE}" \
		./cmd/golangci-lint/
	$(GOLANGLINT) --version
	rm -rf $(TOOL_BUILD_DIR)

$(GOLANGLINT_GOARGS): TOOL_BUILD_DIR := .tmp/build/goargs
$(GOLANGLINT_GOARGS):
	rm -rf $(TOOL_BUILD_DIR)
	git clone https://gitlab.com/gitlab-org/language-tools/go/linters/goargs.git --no-tags --depth 1 $(TOOL_BUILD_DIR)
	cd $(TOOL_BUILD_DIR) && \
	CGO_ENABLED=1 go build --trimpath --buildmode=plugin -o $(GOLANGLINT_GOARGS) plugin/analyzer.go
	rm -rf $(TOOL_BUILD_DIR)

.PHONY: $(MOCKERY)
$(MOCKERY):
	@go install github.com/vektra/mockery/v3@v$(MOCKERY_VERSION)

$(PROTOC): OS_TYPE ?= $(shell uname -s | tr '[:upper:]' '[:lower:]' | sed 's/darwin/osx/')
$(PROTOC): ARCH_SUFFIX = $(if $(findstring osx,$(OS_TYPE)),universal_binary,x86_64)
$(PROTOC): DOWNLOAD_URL = https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(OS_TYPE)-$(ARCH_SUFFIX).zip
$(PROTOC): TOOL_BUILD_DIR = $(local)/build
$(PROTOC):
	# Installing $(DOWNLOAD_URL) as $(PROTOC)
	@mkdir -p $(shell dirname $(PROTOC))
	@mkdir -p "$(TOOL_BUILD_DIR)"
	@curl -sL "$(DOWNLOAD_URL)" -o "$(TOOL_BUILD_DIR)/protoc.zip"
	@unzip "$(TOOL_BUILD_DIR)/protoc.zip" -d "$(TOOL_BUILD_DIR)/"
	# Moving $(TOOL_BUILD_DIR)/bin/protoc to $(PROTOC)
	@mv "$(TOOL_BUILD_DIR)/bin/protoc" "$(PROTOC)"
	@rm -rf "$(TOOL_BUILD_DIR)"
	# Making $(PROTOC) executable
	@chmod +x "$(PROTOC)"

.PHONY: $(PROTOC_GEN_GO)
$(PROTOC_GEN_GO):
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)

.PHONY: $(PROTOC_GEN_GO_GRPC)
$(PROTOC_GEN_GO_GRPC):
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)


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

print_test_ldflags:
	@echo $(GO_TEST_LDFLAGS)

print_image_tags:
	@tags="$(REVISION)"; \
	[ "$(CI_PROJECT_PATH)" = "gitlab-org/gitlab-runner" ] && [ -n "$(CI_COMMIT_TAG)" ] && tags="$$tags $$CI_COMMIT_TAG"; \
	[ "$(IS_LATEST)" = "true" ] && tags="$$tags latest"; \
	[ "$(CI_PROJECT_PATH)" = "gitlab-org/gitlab-runner" ] && ( \
		[ "$(CI_COMMIT_BRANCH)" = "$(CI_DEFAULT_BRANCH)" ] || \
		echo "$(CI_COMMIT_REF_NAME)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+-rc[0-9]+$$' \
	) && tags="$$tags bleeding"; \
	echo "$$tags"

.PHONY: tools # Install dev tool and dependency binaries for local development.
tools: $(GITLAB_CHANGELOG) $(GOCOVER_COBERTURA) $(GOLANGLINT) $(GOLANGLINT_GOARGS) $(MAGE) $(MOCKERY) $(PROTOC) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) $(RELEASE_INDEX_GENERATOR)
