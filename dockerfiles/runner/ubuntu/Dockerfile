ARG BASE_IMAGE

FROM $BASE_IMAGE

ARG TARGETPLATFORM
ARG TARGETARCH
ARG ARCH=${TARGETARCH}

ENV DEBIAN_FRONTEND=noninteractive
# hadolint ignore=DL3008
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends \
        apt-transport-https \
        ca-certificates \
        curl \
        git git-lfs \
        wget \
        tzdata \
        openssh-client \
    && rm -rf /var/lib/apt/lists/*

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION

COPY gitlab-runner_*.deb checksums-* install-deps install-gitlab-runner /tmp/
RUN /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}"
RUN rm -rf /tmp/* /etc/gitlab-runner/.runner_system_id

FROM $BASE_IMAGE

COPY --from=0 / /
COPY --chmod=755 entrypoint /

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
