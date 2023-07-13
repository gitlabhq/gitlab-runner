ARG UBI_VERSION

FROM redhat/ubi8-minimal:${UBI_VERSION}

ARG PLATFORM_ARCH=amd64

RUN microdnf update -y && \
    microdnf install -y --setopt=tsflags=nodocs \
            openssl \
            curl \
            wget \
            openssh-clients \
            hostname \
            procps-ng \
            tar \
            gcc \
            openssl-devel \
            gzip \
            libcurl-devel \
            expat-devel \
            zlib-devel \
            perl-CPAN \
            perl-devel \
            autoconf \
            which \
            gettext \
            diffutils \
            rpm-build && \
    microdnf clean all -y && \
    rm -rf /var/cache/yum

ARG GIT_VERSION

RUN wget https://github.com/git/git/archive/refs/tags/v${GIT_VERSION}.tar.gz && \
    tar xf v${GIT_VERSION}.tar.gz && \
    cd git-${GIT_VERSION} && \
    make configure && \
    ./configure --prefix=/usr/local && \
    NO_TCLTK=1 make all && \
    make install && \
    git --version && \
    rm -rf /git-${GIT_VERSION} && \
    microdnf remove autoconf emacs-filesystem

ARG GIT_LFS_VERSION
# Build git-lfs from source. This is necessary to resolve a number of CVES
# vulnerabilties reported against this image.
#
# We can probably remove this on the next release of git-lfs.
# See https://gitlab.com/gitlab-org/gitlab-runner/-/issues/31065
RUN microdnf install -y --setopt=tsflags=nodocs go
COPY build_git_lfs /tmp/
RUN /tmp/build_git_lfs && \
      microdnf remove go

RUN cd /tmp && \
    git clone https://github.com/larsks/fakeprovide.git && \
    cd fakeprovide && \
    git checkout e26667092bb03bb93d4066b4b10447bbdd1b0d23 && \
    make install && \
    fakeprovide git -v ${GIT_VERSION} && \
    rpm -i fakeprovide-git-*.rpm && \
    rm -rf /tmp/* /usr/bin/fakeprovide

