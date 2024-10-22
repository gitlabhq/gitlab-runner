ARG GO_VERSION=1.21

FROM go-fips-17-0:${GO_VERSION}

WORKDIR /build
COPY . /build/

ARG GOOS=linux
ARG GOARCH=amd64

RUN BASE_DIR="out/binaries/gitlab-runner-helper" && \
    make "${BASE_DIR}/gitlab-runner-helper-fips" GOOS=${GOOS} GOARCH=${GOARCH} && \
    ls "${BASE_DIR}"| grep gitlab-runner-helper| xargs -I '{}' mv "${BASE_DIR}/{}" /gitlab-runner-helper-fips
