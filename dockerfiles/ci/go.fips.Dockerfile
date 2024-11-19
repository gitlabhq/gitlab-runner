ARG GO_FIPS_BASE_IMAGE

FROM ${GO_FIPS_BASE_IMAGE} AS go

ARG PLATFORM_ARCH=amd64

RUN microdnf update -y && \
    microdnf install -y --setopt=tsflags=nodocs openssl-devel glibc-devel tar gzip gcc make git git-lfs && \
    microdnf clean all -y

ARG GO_VERSION=1.23

RUN curl -LO https://go.dev/dl/go${GO_VERSION}.linux-${PLATFORM_ARCH}.tar.gz && \
    tar -C /usr/ -xzf go${GO_VERSION}.linux-${PLATFORM_ARCH}.tar.gz

ENV PATH="$PATH:/usr/go/bin"

ARG GO_RELEASE_VERSION=${GO_VERSION}
ARG GO_FIPS_VERSION_SUFFIX=${GO_FIPS_VERSION_SUFFIX}
RUN git clone \
    https://github.com/golang-fips/go \
    --branch go-${GO_RELEASE_VERSION}${GO_FIPS_VERSION_SUFFIX} \
    --depth 1 \
    /tmp/go

RUN cd /tmp/go && \
    chmod +x scripts/* && \
    git config --global user.email "you@example.com" && \
    git config --global user.name "Your Name" && \
    scripts/full-initialize-repo.sh && \
    pushd go/src && \
    CGO_ENABLED=1 ./make.bash && \
    popd && \
    mv go /usr/local/

RUN cd /usr/local/go/src && \
    rm -rf \
        /usr/local/go/pkg/*/cmd \
        /usr/local/go/pkg/bootstrap \
        /usr/local/go/pkg/obj \
        /usr/local/go/pkg/tool/*/api \
        /usr/local/go/pkg/tool/*/go_bootstrap \
        /usr/local/go/src/cmd/dist/dist \
        /usr/local/go/.git*

FROM ${GO_FIPS_BASE_IMAGE}

RUN microdnf update -y && \
    microdnf install -y patch gcc openssl openssl-devel make git && \
    microdnf clean all -y

COPY --from=go /usr/local/go /usr/local/go
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH" && go install std
WORKDIR $GOPATH
