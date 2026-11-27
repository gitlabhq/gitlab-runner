ARG BASE_IMAGE

FROM $BASE_IMAGE

ARG TARGETOS
ARG TARGETARCH
ARG SRC_SUFFIX=""

# binary_dir and packaging_dir are docker buildx bake named contexts, not stages
# hadolint ignore=DL3022
COPY --from=binary_dir gitlab-runner-${TARGETOS}-${TARGETARCH}${SRC_SUFFIX} /usr/bin/gitlab-runner
# hadolint ignore=DL3022
COPY --from=packaging_dir clear-docker-cache /usr/share/gitlab-runner/
