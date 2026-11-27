ARG BASE_IMAGE

FROM $BASE_IMAGE

ARG TARGETOS
ARG TARGETARCH
ARG SRC_SUFFIX=""
ARG DST_SUFFIX=""
ARG DST_DIR="/usr/bin"

# binary_dir is a docker buildx bake named context, not a stage
# hadolint ignore=DL3022
COPY --from=binary_dir gitlab-runner-helper.${TARGETOS}-${TARGETARCH}${SRC_SUFFIX} ${DST_DIR}/gitlab-runner-helper${DST_SUFFIX}
