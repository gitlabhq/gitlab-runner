package: package-deps package-prepare package-deb package-rpm

package-deps:
	# Installing packaging dependencies...
	command -v fpm 1>/dev/null && fpm --help 1>/dev/null || gem install rake fpm:1.10.2 --no-document

package-prepare:
	chmod 755 packaging/root/usr/share/gitlab-runner/
	chmod 755 packaging/root/usr/share/gitlab-runner/*

package-deb: package-deps package-prepare
	# Building Debian compatible packages...
	$(MAKE) package-deb-fpm ARCH=amd64 PACKAGE_ARCH=amd64 RUNNER_BINARY=out/binaries/$(NAME)-linux-amd64
	$(MAKE) package-deb-fpm ARCH=386 PACKAGE_ARCH=i386 RUNNER_BINARY=out/binaries/$(NAME)-linux-386
	$(MAKE) package-deb-fpm ARCH=arm PACKAGE_ARCH=aarch64 RUNNER_BINARY=out/binaries/$(NAME)-linux-arm64
	$(MAKE) package-deb-fpm ARCH=arm PACKAGE_ARCH=arm64 RUNNER_BINARY=out/binaries/$(NAME)-linux-arm64
	$(MAKE) package-deb-fpm ARCH=arm PACKAGE_ARCH=armel RUNNER_BINARY=out/binaries/$(NAME)-linux-arm
	$(MAKE) package-deb-fpm ARCH=arm PACKAGE_ARCH=armhf RUNNER_BINARY=out/binaries/$(NAME)-linux-arm

package-rpm: package-deps package-prepare
	# Building RedHat compatible packages...
	$(MAKE) package-rpm-fpm ARCH=amd64 PACKAGE_ARCH=amd64 RUNNER_BINARY=out/binaries/$(NAME)-linux-amd64
	$(MAKE) package-rpm-fpm ARCH=386 PACKAGE_ARCH=i686 RUNNER_BINARY=out/binaries/$(NAME)-linux-386
	$(MAKE) package-rpm-fpm ARCH=arm PACKAGE_ARCH=aarch64 RUNNER_BINARY=out/binaries/$(NAME)-linux-arm64
	$(MAKE) package-rpm-fpm ARCH=arm PACKAGE_ARCH=arm64 RUNNER_BINARY=out/binaries/$(NAME)-linux-arm64
	$(MAKE) package-rpm-fpm ARCH=arm PACKAGE_ARCH=arm RUNNER_BINARY=out/binaries/$(NAME)-linux-arm
	$(MAKE) package-rpm-fpm ARCH=arm PACKAGE_ARCH=armhf RUNNER_BINARY=out/binaries/$(NAME)-linux-arm

package-deb-fpm:
	@./ci/package deb

package-rpm-fpm:
	@./ci/package rpm
