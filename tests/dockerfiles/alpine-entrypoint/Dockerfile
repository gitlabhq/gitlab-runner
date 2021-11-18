FROM alpine:latest

run apk --no-cache add su-exec

COPY tests/dockerfiles/alpine-entrypoint/entrypoint.sh /entrypoint

ENTRYPOINT ["/entrypoint"]
