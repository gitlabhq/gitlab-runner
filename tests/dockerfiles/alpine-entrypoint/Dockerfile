FROM alpine:3.24

# Alpine removes superseded package versions, so pinned versions break the build
# hadolint ignore=DL3018
RUN apk --no-cache add su-exec

COPY alpine-entrypoint/entrypoint.sh /entrypoint

ENTRYPOINT ["/entrypoint"]
