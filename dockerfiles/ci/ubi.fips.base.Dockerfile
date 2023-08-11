ARG UBI_VERSION

FROM redhat/ubi8-minimal:${UBI_VERSION} AS git_lfs

ARG GIT_LFS_VERSION
# Build git-lfs from source. This is necessary to resolve a number of CVES
# vulnerabilties reported against this image.
#
# We can probably remove this on the next release of git-lfs.
# See https://gitlab.com/gitlab-org/gitlab-runner/-/issues/31065
COPY dockerfiles/ci/build_git_lfs /tmp/

RUN microdnf update -y && \
    microdnf install -y --setopt=tsflags=nodocs \
        wget make findutils git tar gzip go && \
    /tmp/build_git_lfs

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

RUN microdnf update --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs && \
    microdnf install --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs \
        expat # git runtime dep

COPY --from=git /usr/local/ /usr/local/
COPY --from=git /tmp/fakeprovide-git-*.rpm /tmp
COPY --from=git_lfs /usr/bin/git-lfs /usr/bin

RUN git version && \
    git-lfs install --skip-repo && \
    rpm -i /tmp/fakeprovide-git-*.rpm && \
    rm -fr /tmp/*
