ARG BASE_IMAGE

FROM $BASE_IMAGE

ARG TARGETPLATFORM
ARG ARCH

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION
ARG GIT_LFS_VERSION

COPY gitlab-runner_*.rpm checksums-* install-deps install-gitlab-runner /tmp/
RUN microdnf install --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs \
        tar openssl
RUN /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}"
RUN rm -rf /tmp/* /etc/gitlab-runner/.runner_system_id

FROM $BASE_IMAGE

COPY --from=0 / /
COPY --chmod=755 entrypoint /

ENV FIPS_ENABLED=1

RUN microdnf install --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs \
        openssl # required by gitlab-runner

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
