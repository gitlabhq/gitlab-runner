ARG UBI_VERSION

FROM redhat/ubi8:${UBI_VERSION}

RUN INSTALL_PKGS="openssl-devel glibc-devel gcc git golang" &&  \
    dnf update -y && \
    dnf install -y --setopt=tsflags=nodocs $INSTALL_PKGS && \
    dnf clean all -y

ARG GO_VERSION=1.18

RUN git clone \
    https://github.com/golang-fips/go \
    --branch go${GO_VERSION}-openssl-fips \
    --single-branch \
    --depth 1 \
    /usr/local/go

RUN cd /usr/local/go/src && \
    CGO_ENABLED=1 ./make.bash && \
    rm -rf \
        /usr/local/go/pkg/*/cmd \
        /usr/local/go/pkg/bootstrap \
        /usr/local/go/pkg/obj \
        /usr/local/go/pkg/tool/*/api \
        /usr/local/go/pkg/tool/*/go_bootstrap \
        /usr/local/go/src/cmd/dist/dist \
        /usr/local/go/.git*

FROM redhat/ubi8:${UBI_VERSION}

RUN dnf update -y && \
    dnf install -y patch gcc openssl openssl-devel make git && \
    dnf clean all -y

COPY --from=0 /usr/local/go /usr/local/go
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH" && go install std
WORKDIR $GOPATH
