ARG UBI_VERSION

FROM redhat/ubi8-minimal:${UBI_VERSION} AS git_lfs

ARG GIT_LFS_VERSION
COPY dockerfiles/install_git_lfs /tmp/

ARG ARCH=amd64
ARG GIT_LFS_VERSION
RUN microdnf update --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs && \
    microdnf install --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs \
        wget git tar gzip
RUN /tmp/install_git_lfs

FROM redhat/ubi8-minimal:${UBI_VERSION} AS git

RUN microdnf update --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs && \
    microdnf install --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs \
        autoconf curl-devel expat-devel gettext openssl-devel findutils zlib-devel tar gzip make gcc rpm-build

ARG GIT_VERSION

RUN curl -LO https://github.com/git/git/archive/refs/tags/v${GIT_VERSION}.tar.gz && \
    tar xf v${GIT_VERSION}.tar.gz && \
    cd git-${GIT_VERSION} && \
    make configure && \
    ./configure --prefix=/usr/local && \
    NO_TCLTK=1 NO_PERL=1 NO_PYTHON=1 NO_GETTEXT=1 make all && \
    make install && \
    git --version
RUN cd /tmp && \
    git clone https://github.com/larsks/fakeprovide.git && \
    cd fakeprovide && \
    git checkout e26667092bb03bb93d4066b4b10447bbdd1b0d23 && \
    make install && \
    fakeprovide git -v ${GIT_VERSION} && \
    mv fakeprovide-git-*.rpm /tmp

FROM redhat/ubi8-minimal:${UBI_VERSION}

# these packages are required by downstream images, but we don't know anymore specifically which images require which
# packages, so we'll install them all here...
RUN microdnf update --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs && \
    microdnf install --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs \
    hostname procps wget tar gzip ca-certificates tzdata openssl shadow-utils findutils \
    expat # git runtime dep

COPY --from=git /usr/local/ /usr/local/
COPY --from=git /tmp/fakeprovide-git-*.rpm /tmp
COPY --from=git_lfs /usr/bin/git-lfs /usr/bin

RUN git version && \
    git-lfs install --skip-repo && \
    rpm -i /tmp/fakeprovide-git-*.rpm && \
    rm -fr /tmp/*
