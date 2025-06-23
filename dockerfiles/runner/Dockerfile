ARG BASE_IMAGE

FROM $BASE_IMAGE

ARG TARGETOS
ARG TARGETARCH
ARG SRC_SUFFIX=""

COPY --from=binary_dir gitlab-runner-${TARGETOS}-${TARGETARCH}${SRC_SUFFIX} /usr/bin/gitlab-runner
COPY --from=packaging_dir clear-docker-cache /usr/share/gitlab-runner/
