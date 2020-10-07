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

.PHONY: package-deb
package-deb: package-deps package-prepare
	# Building Debian compatible packages...
	$(MAKE) package-deb-arch ARCH=amd64 PACKAGE_ARCH=amd64
	$(MAKE) package-deb-arch ARCH=386 PACKAGE_ARCH=i386
	$(MAKE) package-deb-arch ARCH=arm64 PACKAGE_ARCH=aarch64
	$(MAKE) package-deb-arch ARCH=arm64 PACKAGE_ARCH=arm64
	$(MAKE) package-deb-arch ARCH=arm PACKAGE_ARCH=armel
	$(MAKE) package-deb-arch ARCH=arm PACKAGE_ARCH=armhf
	$(MAKE) package-deb-arch ARCH=s390x PACKAGE_ARCH=s390x

.PHONY: package-rpm
package-rpm: package-deps package-prepare
	# Building RedHat compatible packages...
	$(MAKE) package-rpm-arch ARCH=amd64 PACKAGE_ARCH=amd64
	$(MAKE) package-rpm-arch ARCH=386 PACKAGE_ARCH=i686
	$(MAKE) package-rpm-arch ARCH=arm64 PACKAGE_ARCH=aarch64
	$(MAKE) package-rpm-arch ARCH=arm64 PACKAGE_ARCH=arm64
	$(MAKE) package-rpm-arch ARCH=arm PACKAGE_ARCH=arm
	$(MAKE) package-rpm-arch ARCH=arm PACKAGE_ARCH=armhf
	$(MAKE) package-rpm-arch ARCH=s390x PACKAGE_ARCH=s390x

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
