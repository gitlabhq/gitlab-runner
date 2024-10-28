ARG UBI_MICRO_IMAGE=redhat/ubi9-micro
ARG UBI_MINIMAL_IMAGE=redhat/ubi9-minimal

FROM ${UBI_MICRO_IMAGE} AS initial

FROM ${UBI_MINIMAL_IMAGE} AS build
ARG DNF_OPTS_ROOT="--installroot=/install-root/ --noplugins  --setopt=reposdir=/install-root/etc/yum.repos.d/ \
    --setopt=varsdir=/install-var/ --config= --setopt=cachedir=/install-cache/"

RUN mkdir -p /install-root/ /install-var
COPY --from=initial / /install-root/

# these packages are required by downstream images, but we don't know anymore specifically which images require which
RUN microdnf update ${DNF_OPTS_ROOT} --best --refresh --assumeyes --nodocs --setopt=install_weak_deps=0 --setopt=tsflags=nodocs
RUN microdnf install ${DNF_OPTS_ROOT} --best --refresh  --assumeyes --nodocs  --setopt=install_weak_deps=0 --setopt=tsflags=nodocs \
        hostname procps tar gzip ca-certificates tzdata openssl git findutils

# Install git-lfs from upstream rpm repo since ubi9's version is always several releases behind
RUN curl -s https://packagecloud.io/install/repositories/github/git-lfs/script.rpm.sh | bash
# Disable gpgcheck since this package is signed with SHA1 which is no longer supported in rhel.
RUN sed -i 's/gpgcheck=1/gpgcheck=0/g' /etc/yum.repos.d/github_git-lfs.repo
RUN cp /etc/yum.repos.d/github_git-lfs.repo /install-root/etc/yum.repos.d/
RUN microdnf install ${DNF_OPTS_ROOT} --best --refresh  --assumeyes --nodocs  --setopt=install_weak_deps=0 --setopt=tsflags=nodocs git-lfs

RUN microdnf clean  ${DNF_OPTS_ROOT}  all \
    && rm -f /install-root/var/lib/dnf/history*

FROM ${UBI_MICRO_IMAGE} AS final

COPY --from=build  /install-root/ /
RUN git-lfs install --skip-repo \
    && rm -f /var/lib/dnf/history*
