ARG BASE_IMAGE

FROM $BASE_IMAGE

ARG TARGETPLATFORM

# hadolint ignore=DL3008
RUN dnf update -y && \
    dnf install -y \
        openssl \
        curl \
        git \
        wget \
        openssh-clients \
        && dnf clean all && \
        rm -rf /var/cache/dnf

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION
ARG GIT_LFS_VERSION

COPY gitlab-runner_*.rpm checksums-* install-deps install-gitlab-runner /tmp/
RUN /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}" "${GIT_LFS_VERSION}"
RUN rm -rf /tmp/*

FROM $BASE_IMAGE

COPY --from=0 / /
COPY --chmod=777 entrypoint /

ENV FIPS_ENABLED=1

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
