ARG GO_VERSION=1.18
ARG BUILD_IMAGE=go-fips

FROM ${BUILD_IMAGE}:${GO_VERSION}

WORKDIR /build
COPY . /build/

ARG GOOS=linux
ARG GOARCH=amd64

RUN make runner-bin-fips GOOS=${GOOS} GOARCH=${GOARCH} && \
    cp out/binaries/* /
