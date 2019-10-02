package: package-deps package-prepare package-deb package-rpm

package-deps:
	# Installing packaging dependencies...
	which fpm 1>/dev/null || gem install rake fpm:1.10.2 --no-document

package-prepare:
	chmod 755 packaging/root/usr/share/gitlab-runner/
	chmod 755 packaging/root/usr/share/gitlab-runner/*

package-deb: package-deps package-prepare
	# Building Debian compatible packages...
	make package-deb-fpm ARCH=amd64 PACKAGE_ARCH=amd64 RUNNER_BINARY=out/binaries/$(NAME)-linux-amd64
	make package-deb-fpm ARCH=386 PACKAGE_ARCH=i386 RUNNER_BINARY=out/binaries/$(NAME)-linux-386
	make package-deb-fpm ARCH=arm PACKAGE_ARCH=armel RUNNER_BINARY=out/binaries/$(NAME)-linux-arm
	make package-deb-fpm ARCH=arm PACKAGE_ARCH=armhf RUNNER_BINARY=out/binaries/$(NAME)-linux-arm

package-rpm: package-deps package-prepare
	# Building RedHat compatible packages...
	make package-rpm-fpm ARCH=amd64 PACKAGE_ARCH=amd64 RUNNER_BINARY=out/binaries/$(NAME)-linux-amd64
	make package-rpm-fpm ARCH=386 PACKAGE_ARCH=i686 RUNNER_BINARY=out/binaries/$(NAME)-linux-386
	make package-rpm-fpm ARCH=arm PACKAGE_ARCH=arm RUNNER_BINARY=out/binaries/$(NAME)-linux-arm
	make package-rpm-fpm ARCH=arm PACKAGE_ARCH=armhf RUNNER_BINARY=out/binaries/$(NAME)-linux-arm

package-deb-fpm:
	@./ci/package deb

package-rpm-fpm:
	@./ci/package rpm
