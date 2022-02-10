.PHONY: package
package: package-deb package-rpm

.PHONY: package-deps
package-deps:
	# Installing packaging dependencies...
	command -v fpm 1>/dev/null && fpm --help 1>/dev/null || gem install rake fpm:1.10.2 --no-document

.PHONY: package-prepare
package-prepare:
	chmod 755 packaging/root/usr/share/gitlab-runner/
	chmod 755 packaging/root/usr/share/gitlab-runner/*

.PHONY: package-deb-64bit
package-deb-64bit: package-deps package-prepare
	$(MAKE) package-deb-arch ARCH=amd64 PACKAGE_ARCH=amd64

.PHONY: package-deb-arm-64bit
package-deb-arm-64bit: package-deps package-prepare
	$(MAKE) package-deb-arch ARCH=arm64 PACKAGE_ARCH=aarch64
	$(MAKE) package-deb-arch ARCH=arm64 PACKAGE_ARCH=arm64

.PHONY: package-deb-32bit
package-deb-32bit: package-deps package-prepare
	$(MAKE) package-deb-arch ARCH=386 PACKAGE_ARCH=i386

.PHONY: package-deb-arm-32bit
package-deb-arm-32bit: package-deps package-prepare
	$(MAKE) package-deb-arch ARCH=arm PACKAGE_ARCH=armel
	$(MAKE) package-deb-arch ARCH=arm PACKAGE_ARCH=armhf

.PHONY: package-deb-ibm
package-deb-ibm: package-deps package-prepare
	$(MAKE) package-deb-arch ARCH=s390x PACKAGE_ARCH=s390x
	$(MAKE) package-deb-arch ARCH=ppc64le PACKAGE_ARCH=ppc64el

.PHONY: package-rpm-64bit
package-rpm-64bit: package-deps package-prepare
	$(MAKE) package-rpm-arch ARCH=amd64 PACKAGE_ARCH=amd64

.PHONY: package-rpm-arm-64bit
package-rpm-arm-64bit: package-deps package-prepare
	$(MAKE) package-rpm-arch ARCH=arm64 PACKAGE_ARCH=aarch64
	$(MAKE) package-rpm-arch ARCH=arm64 PACKAGE_ARCH=arm64

.PHONY: package-rpm-32bit
package-rpm-32bit: package-deps package-prepare
	$(MAKE) package-rpm-arch ARCH=386 PACKAGE_ARCH=i686

.PHONY: package-rpm-arm-32bit
package-rpm-arm-32bit: package-deps package-prepare
	$(MAKE) package-rpm-arch ARCH=arm PACKAGE_ARCH=arm
	$(MAKE) package-rpm-arch ARCH=arm PACKAGE_ARCH=armhf

.PHONY: package-rpm-ibm
package-rpm-ibm: package-deps package-prepare
	$(MAKE) package-rpm-arch ARCH=s390x PACKAGE_ARCH=s390x
	$(MAKE) package-rpm-arch ARCH=ppc64le PACKAGE_ARCH=ppc64le

.PHONY: package-rpm-fips
package-rpm-fips: ARCH ?= amd64
package-rpm-fips: export PACKAGE_ARCH ?= amd64
package-rpm-fips: export RUNNER_BINARY ?= out/binaries/$(NAME)-linux-$(ARCH)-fips
package-rpm-fips: package-deps package-prepare
	@./ci/package rpm-fips

.PHONY: package-deb-arch
package-deb-arch: ARCH ?= amd64
package-deb-arch: export PACKAGE_ARCH ?= amd64
package-deb-arch: export RUNNER_BINARY ?= out/binaries/$(NAME)-linux-$(ARCH)
package-deb-arch:
	@./ci/package deb

.PHONY: package-rpm-arch
package-rpm-arch: ARCH ?= amd64
package-rpm-arch: export PACKAGE_ARCH ?= amd64
package-rpm-arch: export RUNNER_BINARY ?= out/binaries/$(NAME)-linux-$(ARCH)
package-rpm-arch:
	@./ci/package rpm
