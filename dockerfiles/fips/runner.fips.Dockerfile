ARG GO_VERSION=1.17

FROM go-fips:${GO_VERSION}

WORKDIR /build
COPY . /build/

ARG GOOS=linux
ARG GOARCH=amd64

RUN make runner-bin-fips GOOS=${GOOS} GOARCH=${GOARCH} && \
    cp out/binaries/* /
