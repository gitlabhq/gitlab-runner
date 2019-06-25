FROM ubuntu:18.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y ca-certificates wget apt-transport-https vim nano tzdata git curl && \
    rm -rf /var/lib/apt/lists/*

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION
ARG GIT_LFS_VERSION

COPY gitlab-runner_amd64.deb /tmp/
COPY checksums /tmp/
RUN dpkg -i /tmp/gitlab-runner_amd64.deb; \
    apt-get update &&  \
    apt-get -f install -y && \
    rm -rf /var/lib/apt/lists/* && \
    rm /tmp/gitlab-runner_amd64.deb && \
    gitlab-runner --version && \
    mkdir -p /etc/gitlab-runner/certs && \
    chmod -R 700 /etc/gitlab-runner && \
    wget -nv https://github.com/docker/machine/releases/download/v${DOCKER_MACHINE_VERSION}/docker-machine-Linux-x86_64 -O /usr/bin/docker-machine && \
    chmod +x /usr/bin/docker-machine && \
    docker-machine --version && \
    wget -nv https://github.com/Yelp/dumb-init/releases/download/v${DUMB_INIT_VERSION}/dumb-init_${DUMB_INIT_VERSION}_amd64 -O /usr/bin/dumb-init && \
    chmod +x /usr/bin/dumb-init && \
    dumb-init --version && \
    wget -nv https://github.com/git-lfs/git-lfs/releases/download/v${GIT_LFS_VERSION}/git-lfs-linux-amd64-v${GIT_LFS_VERSION}.tar.gz -O /tmp/git-lfs.tar.gz && \
    mkdir /tmp/git-lfs && \
    tar -xzf /tmp/git-lfs.tar.gz -C /tmp/git-lfs/ && \
    mv /tmp/git-lfs/git-lfs /usr/bin/git-lfs && \
    rm -rf /tmp/git-lfs* && \
    git-lfs install --skip-repo && \
    git-lfs version && \
    sha256sum --check --strict /tmp/checksums

COPY entrypoint /
RUN chmod +x /entrypoint

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
