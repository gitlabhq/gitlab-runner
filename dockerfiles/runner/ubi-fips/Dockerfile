ARG BASE_IMAGE
ARG UBI_MINIMAL_IMAGE

FROM ${BASE_IMAGE} AS initial

FROM ${UBI_MINIMAL_IMAGE} AS build

ARG DNF_INSTALL_ROOT=/install-root
ARG DNF_OPTS_ROOT="--installroot=${DNF_INSTALL_ROOT}/ --setopt=reposdir=${DNF_INSTALL_ROOT}/etc/yum.repos.d/ --setopt=varsdir=/install-var/ --config=${DNF_INSTALL_ROOT}/etc/yum.repos.d/ubi.repo --setopt=cachedir=/install-cache/ --noplugins -y"

ARG TARGETPLATFORM
ARG ARCH

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION
ARG GIT_LFS_VERSION

RUN mkdir -p ${DNF_INSTALL_ROOT}/ /install-var
COPY --from=initial / ${DNF_INSTALL_ROOT}/

COPY gitlab-runner_*.rpm checksums-* install-deps install-gitlab-runner entrypoint /tmp/
RUN /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}"

# https://docs.openshift.com/container-platform/4.6/openshift_images/create-images.html#support-arbitrary-user-ids
RUN install -d -m 770 -o 1001 -g 0 ${DNF_INSTALL_ROOT}/etc/gitlab-runner/certs && \
    install -d -m 770 -o 1001 -g 0 ${DNF_INSTALL_ROOT}/home/gitlab-runner && \
    install -d -m 770 -o 1001 -g 0 ${DNF_INSTALL_ROOT}/secrets

# install entrypoint as root:root r-xr-xr-x
RUN install -m 555 -o 0 -g 0 /tmp/entrypoint ${DNF_INSTALL_ROOT}/entrypoint

RUN rm -rf /tmp/* /etc/gitlab-runner/.runner_system_id

FROM ${BASE_IMAGE} AS final

ARG DNF_INSTALL_ROOT=/install-root

COPY --from=build ${DNF_INSTALL_ROOT}/ /

ENV FIPS_ENABLED=1

ENV HOME /home/gitlab-runner

USER 1001

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
