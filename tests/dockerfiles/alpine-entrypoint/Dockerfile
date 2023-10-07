FROM alpine:latest

RUN apk --no-cache add su-exec

COPY tests/dockerfiles/alpine-entrypoint/entrypoint.sh /entrypoint
COPY tests/dockerfiles/alpine-entrypoint/entrypoint-override.sh /entrypoint-override

ENTRYPOINT ["/entrypoint"]
