ARG BASE_IMAGE

FROM $BASE_IMAGE

ARG TARGETPLATFORM
ARG TARGETARCH
ARG ARCH=${TARGETARCH}

RUN adduser -D -S -h /home/gitlab-runner gitlab-runner

# hadolint ignore=DL3018
RUN apk add --no-cache \
    bash \
    ca-certificates \
    git git-lfs \
    tzdata \
    openssh-client \
    curl

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION

COPY gitlab-runner-linux-* /usr/bin/
COPY checksums-* install-deps install-gitlab-runner /tmp/

# Install GNU wget for "-nv" flag support
# hadolint ignore=DL3018
RUN apk add --no-cache --virtual .fetch-deps wget && \
    /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}" && \
    apk del .fetch-deps
RUN rm -rf /tmp/* /etc/gitlab-runner/.runner_system_id

FROM $BASE_IMAGE

COPY --from=0 / /
COPY --chmod=755 entrypoint /

RUN apk update && apk upgrade --no-cache

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
