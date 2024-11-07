ARG BASE_IMAGE

FROM $BASE_IMAGE

ARG TARGETOS
ARG TARGETARCH

COPY --from=binary_dir gitlab-runner-${TARGETOS}-${TARGETARCH} /usr/bin/gitlab-runner
