GREEN  := $(shell tput -Txterm setaf 2)
RED    := $(shell tput -Txterm setaf 1)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: .deprecation-notice
.deprecation-notice:
	@echo "${YELLOW}----------------------------------------------------------------------------------------------"
	@echo "WARNING: The ${RED}$(_OLD_NAME)${YELLOW} target has been deprecated and replaced with ${GREEN}$(_NEW_NAME)${YELLOW}."
	@echo "         Please consider switching to the new target for future usages."
	@echo "----------------------------------------------------------------------------------------------${RESET}"

.PHONY: build_all
build_all: export _OLD_NAME := "build_all"
build_all: export _NEW_NAME := "runner-bin"
build_all: .deprecation-notice runner-bin
	@$(MAKE) .deprecation-notice

.PHONY: build_simple
build_simple: export _OLD_NAME := "build_simple"
build_simple: export _NEW_NAME := "runner-bin-host"
build_simple: .deprecation-notice runner-bin-host
	@$(MAKE) .deprecation-notice

.PHONY: build_current
build_current: export _OLD_NAME := "build_current"
build_current: export _NEW_NAME := "runner-and-helper-bin-host"
build_current: .deprecation-notice runner-and-helper-bin-host
	@$(MAKE) .deprecation-notice

.PHONY: build_current_docker
build_current_docker: export _OLD_NAME := "build_current_docker"
build_current_docker: export _NEW_NAME := "runner-and-helper-docker-host"
build_current_docker: .deprecation-notice runner-and-helper-docker-host
	@$(MAKE) .deprecation-notice

.PHONY: build_current_deb
build_current_deb: export _OLD_NAME := "build_current_deb"
build_current_deb: export _NEW_NAME := "runner-and-helper-deb-host"
build_current_deb: .deprecation-notice runner-and-helper-deb-host
	@$(MAKE) .deprecation-notice

.PHONY: build_current_rpm
build_current_rpm: export _OLD_NAME := "build_current_rpm"
build_current_rpm: export _NEW_NAME := "runner-and-helper-rpm-host"
build_current_rpm: .deprecation-notice runner-and-helper-rpm-host
	@$(MAKE) .deprecation-notice

.PHONY: package-deb-fpm
package-deb-fpm: export _OLD_NAME := "package-deb-fpm"
package-deb-fpm: export _NEW_NAME := "package-deb-arch"
package-deb-fpm: .deprecation-notice package-deb-arch
	@$(MAKE) .deprecation-notice

.PHONY: package-rpm-fpm
package-rpm-fpm: export _OLD_NAME := "package-rpm-fpm"
package-rpm-fpm: export _NEW_NAME := "package-rpm-arch"
package-rpm-fpm: .deprecation-notice package-rpm-arch
	@$(MAKE) .deprecation-notice

.PHONY: helper-build
helper-build: export _OLD_NAME := "helper-build"
helper-build: export _NEW_NAME := "helper-bin"
helper-build: .deprecation-notice helper-bin
	@$(MAKE) .deprecation-notice

.PHONY: helper-docker
helper-docker: export _OLD_NAME := "helper-docker"
helper-docker: export _NEW_NAME := "helper-dockerarchive"
helper-docker: .deprecation-notice helper-dockerarchive
	@$(MAKE) .deprecation-notice
