FROM alpine:latest

run apk --no-cache add su-exec

COPY tests/dockerfiles/alpine-entrypoint/my-entrypoint.sh /entrypoint

ENTRYPOINT ["/entrypoint"]
