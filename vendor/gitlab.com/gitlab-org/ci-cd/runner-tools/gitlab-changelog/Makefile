export VERSION ?= $(shell (scripts/version 2>/dev/null || echo "dev") | sed -e 's/^v//g')
export REVISION ?= $(shell git rev-parse --short HEAD || echo "unknown")
export BRANCH ?= $(shell git show-ref | grep "$(REVISION)" | grep -v HEAD | awk '{print $$2}' | sed 's|refs/remotes/origin/||' | sed 's|refs/heads/||' | sort | head -n 1)
export BUILT ?= $(shell date -u +%Y-%m-%dT%H:%M:%S%z)
export CGO_ENABLED ?= 0
export GOPATH ?= ".go/"
export BUILD_PLATFORMS ?= -osarch 'linux/amd64' -osarch 'darwin/amd64'

export goJunitReport ?= $(GOPATH)/bin/go-junit-report
mockery ?= $(GOPATH)/bin/mockery
gox ?= $(GOPATH)/bin/gox

RELEASE_INDEX_GEN_VERSION ?= master
releaseIndexGen ?= .tmp/release-index-gen-$(RELEASE_INDEX_GEN_VERSION)

PKG := $(shell go list .)
PKGs := $(shell go list ./... | grep -vE "^/vendor/")

GO_LDFLAGS := -X $(PKG).VERSION=$(VERSION) \
              -X $(PKG).REVISION=$(REVISION) \
              -X $(PKG).BRANCH=$(BRANCH) \
              -X $(PKG).BUILT=$(BUILT) \
              -s -w

.PHONY: compile
compile:
	go build \
			-o build/gitlab-changelog \
			-ldflags "$(GO_LDFLAGS)" \
			./cmd/gitlab-changelog

.PHONY: compile_all
compile_all: $(gox)
	# Building project in version $(VERSION) for $(BUILD_PLATFORMS)
	$(gox) $(BUILD_PLATFORMS) \
			-ldflags "$(GO_LDFLAGS)" \
			-output="build/gitlab-changelog-{{.OS}}-{{.Arch}}" \
			./cmd/gitlab-changelog

export testsDir = ./.tests

.PHONY: tests
tests: $(testsDir) $(goJunitReport)
	@./scripts/tests normal

.PHONY: tests_race
tests_race: $(testsDir) $(goJunitReport)
	@./scripts/tests race

.PHONY: codequality
# Using codeclimate linters forks, because the official releases are built on top of old versions of Go,
# which generates errors, e.g. when sing new error handling implemented in stdlib `errors` package in Go 1.13.
# We've prepared updates and pushed patches to the upstream, but until they will be merged and released
# the only way is to use the forks.
codequality: CODECLIMATE_VERSION ?= 0.85.5
codequality: REPORT_FILE ?= gl-code-quality-report.json
codequality:
	# Pulling gocyclo analyzer
	@docker pull registry.gitlab.com/tmaczukin/codeclimate-gocyclo:tm-go1.12.5-alpine3.9-1 > /dev/null
	@docker tag registry.gitlab.com/tmaczukin/codeclimate-gocyclo:tm-go1.12.5-alpine3.9-1 codeclimate/codeclimate-gocyclo > /dev/null
	# Pulling gofmt analyzer
	@docker pull registry.gitlab.com/tmaczukin/codeclimate-gofmt:tm-go1.12.5-alpine3.9-2 > /dev/null
	@docker tag registry.gitlab.com/tmaczukin/codeclimate-gofmt:tm-go1.12.5-alpine3.9-2 codeclimate/codeclimate-gofmt > /dev/null
	# Pulling govet analyzer
	@docker pull registry.gitlab.com/tmaczukin/codeclimate-govet:tm-go1.12.5-alpine3.9-2 > /dev/null
	@docker tag registry.gitlab.com/tmaczukin/codeclimate-govet:tm-go1.12.5-alpine3.9-2 codeclimate/codeclimate-govet > /dev/null
	# Pulling golint analyzer
	@docker pull registry.gitlab.com/tmaczukin/codeclimate-golint:tm-go1.12.5-alpine3.9-2 > /dev/null
	@docker tag registry.gitlab.com/tmaczukin/codeclimate-golint:tm-go1.12.5-alpine3.9-2 codeclimate/codeclimate-golint > /dev/null
	# Starting codeclimate
	@docker run --rm --env CODECLIMATE_CODE="$$(pwd)" \
		--volume "$$(pwd)":/code \
		--volume /var/run/docker.sock:/var/run/docker.sock \
		--volume /tmp/cc:/tmp/cc \
		codeclimate/codeclimate:$(CODECLIMATE_VERSION) \
		analyze -f json --dev | tee $(REPORT_FILE)
	# Checking if offenses are reported
	@if [ "$$(cat $(REPORT_FILE))" != "[]" ] ; then \
	  jq -C . $(REPORT_FILE); \
	  exit 1 ;\
	fi

$(testsDir):
	# Preparing tests output directory
	@mkdir -p $@

.PHONY: fmt
fmt:
	# Fixing project code formatting...
	@go fmt $(PKGs) | awk '{if (NF > 0) {if (NR == 1) print "Please run go fmt for:"; print "- "$$1}} END {if (NF > 0) {if (NR > 0) exit 1}}'

.PHONY: mocks
mocks: $(mockery)
	# Removing existing mocks
	@find * -name "mock_*.go" -delete
	# Generating new mocks
	@$(mockery) -recursive -all -inpkg -dir ./

.PHONY: check_mocks
check_mocks:
	# Checking if mocks are up-to-date
	# Saving the list of committed mocks
	@git status -sb > /tmp/mocks-$${CI_JOB_ID}-before
	@$(MAKE) mocks
	# Saving the list of created mocks
	@git status -sb > /tmp/mocks-$${CI_JOB_ID}-after
	# Checking the difference
	@diff -U0 /tmp/mocks-$${CI_JOB_ID}-before /tmp/mocks-$${CI_JOB_ID}-after

.PHONY: check_modules
check_modules:
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-before
	@go mod tidy
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-after
	@diff -U0 /tmp/gosum-$${CI_JOB_ID}-before /tmp/gosum-$${CI_JOB_ID}-after

.PHONY: prepare_ci_image
prepare_ci_image: CI_IMAGE ?= gitlab-changelog
prepare_ci_image: CI_REGISTRY ?= ""
prepare_ci_image:
	# Builiding the $(CI_IMAGE) image
	@docker build \
			--pull \
			--no-cache \
			--build-arg GO_VERSION=$${GO_VERSION} \
			--build-arg ALPINE_VERSION=$${ALPINE_VERSION} \
			--build-arg GCLOUD_SDK_VERSION=$${GCLOUD_SDK_VERSION} \
			-t $(CI_IMAGE) \
			-f dockerfiles/ci/Dockerfile \
			dockerfiles/ci/
ifneq ($(CI_REGISTRY),)
	# Pushing the $(CI_IMAGE) image to $(CI_REGISTRY)
	@docker login --username $${CI_REGISTRY_USER} --password $${CI_REGISTRY_PASSWORD} $(CI_REGISTRY)
	@docker push $(CI_IMAGE)
	@docker logout $(CI_REGISTRY)
else
	# No CI_REGISTRY value, skipping image push
endif

.PHONY: release_gcs
release_gcs: CI_COMMIT_REF_NAME ?= $(BRANCH)
release_gcs: CI_COMMIT_SHA ?= $(REVISION)
release_gcs: GCS_BUCKET ?=
release_gcs: GCS_PATH ?=
release_gcs: latest_stable_tag := $(shell git -c versionsort.prereleaseSuffix="-rc" tag -l "v*.*.*" --sort=-v:refname | awk '!/rc/' | head -n 1)
release_gcs:
	# Preparing index page
	@$(MAKE) index_file
ifneq ($(GCS_BUCKET),)
	@$(MAKE) sync_gcs_release GCS_URL="gs://$(GCS_BUCKET)/$(GCS_PATH)/$(CI_COMMIT_REF_NAME)/"
	@if [ $$(git describe --exact-match --match $(latest_stable_tag) >/dev/null 2>&1) ]; then \
			@$(MAKE) sync_gcs_release GCS_URL="gs://$(GCS_BUCKET)/$(GCS_PATH)/latest/"; \
	fi
	@$(MAKE) release_gitlab
endif

.PHONY: sync_gcs_release
sync_gcs_release: GCS_URL ?=
sync_gcs_release:
	# Syncing with $(GCS_URL)
	@gsutil rsync build/ "$(GCS_URL)"
	@gsutil acl set -r public-read "$(GCS_URL)"

.PHONY: remove_gcs_release
remove_gcs_release: CI_COMMIT_REF_NAME ?= $(BRANCH)
remove_gcs_release: GCS_BUCKET ?=
remove_gcs_release: GCS_PATH ?=
remove_gcs_release:
ifneq ($(GCS_BUCKET),)
	@gsutil rm -r "gs://$(GCS_BUCKET)/$(GCS_PATH)/$(CI_COMMIT_REF_NAME)"
endif

.PHONY: release_gitlab
release_gitlab: export CI_COMMIT_TAG ?=
release_gitlab: export CI_PROJECT_URL ?=
release_gitlab:
ifneq ($(CI_COMMIT_TAG),)
	# Saving as GitLab release at $(CI_PROJECT_URL)/-/releases
	@./scripts/gitlab_release
endif

.PHONY: index_file
index_file: export CI_COMMIT_REF_NAME ?= $(BRANCH)
index_file: export CI_COMMIT_SHA ?= $(REVISION)
index_file: $(releaseIndexGen)
	# generating index.html file
	@$(releaseIndexGen) \
		-working-directory build/ \
		-project-version $(VERSION) \
		-project-git-ref $(CI_COMMIT_REF_NAME) \
		-project-git-revision $(CI_COMMIT_SHA) \
		-project-name "GitLab Changelog generator" \
		-project-repo-url "https://gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog" \
		-gpg-key-env GPG_KEY \
		-gpg-password-env GPG_PASSPHRASE

.PHONY: generate_changelog
generate_changelog: export CHANGELOG_RELEASE ?= $(VERSION)
generate_changelog: build/gitlab-changelog
	@build/gitlab-changelog \
		-changelog-file CHANGELOG.md \
		-config-file .gitlab/changelog.yml \
		-project-id 17192985 \
		-release $(CHANGELOG_RELEASE) \
		-starting-point-matcher "v[0-9]*.[0-9]*.[0-9]*"

build/gitlab-changelog:
	@$(MAKE) compile

$(mockery):
	# Installing $(mockery)
	@go install github.com/vektra/mockery/cmd/mockery

$(goJunitReport):
	# Installing $(goJunitReport)
	@go install github.com/jstemmer/go-junit-report

$(gox):
	# Installing $(gox)
	@go install github.com/mitchellh/gox

$(releaseIndexGen): OS_TYPE ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
$(releaseIndexGen): DOWNLOAD_URL = "https://storage.googleapis.com/gitlab-runner-tools/release-index-generator/$(RELEASE_INDEX_GEN_VERSION)/release-index-gen-$(OS_TYPE)-amd64"
$(releaseIndexGen):
	# Installing $(DOWNLOAD_URL) as $(releaseIndexGen)
	@mkdir -p $(shell dirname $(releaseIndexGen))
	@curl -sL "$(DOWNLOAD_URL)" -o "$(releaseIndexGen)"
	@chmod +x "$(releaseIndexGen)"
