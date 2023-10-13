ARG UBI_VERSION

FROM redhat/ubi8-minimal:${UBI_VERSION}

# these packages are required by downstream images, but we don't know anymore specifically which images require which
# packages, so we'll install them all here...
RUN microdnf update --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs && \
    microdnf install --best --refresh --assumeyes --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=tsflags=nodocs \
    hostname procps wget tar gzip ca-certificates tzdata openssl shadow-utils git git-lfs findutils

RUN git-lfs install --skip-repo
