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
        hostname \
        procps-ng \
        && dnf clean all && \
        rm -rf /var/cache/dnf

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION
ARG GIT_LFS_VERSION

COPY gitlab-runner_*.rpm checksums-* install-deps install-gitlab-runner /tmp/
RUN /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}" "${GIT_LFS_VERSION}"
RUN rm -rf /tmp/* /etc/gitlab-runner/.runner_system_id

FROM $BASE_IMAGE

COPY --from=0 / /
COPY --chmod=777 entrypoint /

ENV FIPS_ENABLED=1

ENV HOME /home/gitlab-runner

# https://docs.openshift.com/container-platform/4.6/openshift_images/create-images.html#support-arbitrary-user-ids
RUN mkdir -p /etc/gitlab-runner/certs $HOME /secrets && \
    chgrp -R 0 /etc/gitlab-runner && \
    chmod -R g=u /etc/gitlab-runner && \
    chgrp -R 0 $HOME && \
    chmod -R g=u $HOME && \
    chgrp -R 0 /secrets && \
    chmod -R g=u /secrets

USER 1001

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
