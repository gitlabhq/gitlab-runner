FROM alpine:3.10

ARG TARGETPLATFORM

RUN adduser -D -S -h /home/gitlab-runner gitlab-runner

RUN apk add --no-cache \
    bash \
    ca-certificates \
    git \
    openssl \
    tzdata \
    wget

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION
ARG GIT_LFS_VERSION

COPY gitlab-runner-linux-* /usr/bin/
COPY checksums-* install-deps install-gitlab-runner /tmp/
RUN /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}" "${GIT_LFS_VERSION}"

COPY entrypoint /
RUN chmod +x /entrypoint

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
