# renovate: datasource=docker depName=golang allowedVersions=/1\.18\..+\-buster/
FROM golang:1.18.10-buster

# Install required dependencies
# hadolint ignore=DL3008,DL3015
RUN apt-get update -yq && \
    apt-get install -yq locales make xz-utils \
                        ruby ruby-dev python-pip \
                        dpkg-sig createrepo rpm \
                        zip libffi-dev jq zip && \
    rm -rf /var/lib/apt/lists/*

# Set default locale for the environment
RUN echo "en_US UTF-8" > /etc/locale.gen; \
    locale-gen en_US.UTF-8

ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8

# Install docker client
ARG DOCKER_VERSION
RUN wget -q https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz -O /tmp/docker.tar.gz && \
    tar -xzf /tmp/docker.tar.gz -C /tmp/ && \
    cp /tmp/docker/docker* /usr/bin && \
    chmod +x /usr/bin/docker* && \
    rm -rf /tmp/*

# Install Powershell Core
ARG PWSH_VERSION
RUN wget https://packages.microsoft.com/config/debian/10/packages-microsoft-prod.deb && \
    dpkg -i packages-microsoft-prod.deb && \
    rm packages-microsoft-prod.deb && \
    apt-get update && \
    apt-get install -y powershell=${PWSH_VERSION}-1.debian.10 && \
    rm -rf /var/lib/apt/lists/* && \
    pwsh --version

# Install docker buildx plugin
ARG BUILDX_VERSION
RUN mkdir -p ~/.docker/cli-plugins && \
    wget -q https://github.com/docker/buildx/releases/download/v${BUILDX_VERSION}/buildx-v${BUILDX_VERSION}.linux-amd64 -O ~/.docker/cli-plugins/docker-buildx && \
    chmod a+x ~/.docker/cli-plugins/docker-buildx

ARG GIT_LFS_VERSION
ARG GIT_LFS_AMD64_CHECKSUM

COPY dockerfiles/ci/install_git_lfs /tmp/
RUN /tmp/install_git_lfs

# Install kubectl
ARG KUBECTL_VERSION
RUN wget -q https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl -O /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

# Install aws-cli
ARG AWS_CLI_VERSION
RUN wget -q https://awscli.amazonaws.com/awscli-exe-linux-x86_64-${AWS_CLI_VERSION}.zip -O awscliv2.zip && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -r aws && \
    rm awscliv2.zip

# Install yq
ARG YQ_VERSION
RUN wget https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64 -O /usr/bin/yq &&\
        chmod +x /usr/bin/yq

COPY Makefile* /tmp/
WORKDIR /tmp
RUN make deps package-deps packagecloud-deps

RUN go install github.com/jstemmer/go-junit-report@2a37e36ac67395cc83bca97552a19c8a4ec4ddb1
