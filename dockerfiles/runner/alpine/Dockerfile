FROM alpine:3.13.6

ARG TARGETPLATFORM

RUN adduser -D -S -h /home/gitlab-runner gitlab-runner

# hadolint ignore=DL3018
RUN apk add --no-cache \
    bash \
    ca-certificates \
    git \
    tzdata \
    openssh-client

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION
ARG GIT_LFS_VERSION

COPY gitlab-runner-linux-* /usr/bin/
COPY checksums-* install-deps install-gitlab-runner /tmp/

# Install GNU wget for "-nv" flag support
# hadolint ignore=DL3018
RUN apk add --no-cache --virtual .fetch-deps wget && \
    /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}" "${GIT_LFS_VERSION}" && \
    apk del .fetch-deps

COPY entrypoint /
RUN chmod +x /entrypoint

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
