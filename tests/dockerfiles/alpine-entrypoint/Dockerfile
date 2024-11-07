FROM alpine:latest

RUN apk --no-cache add su-exec

COPY alpine-entrypoint/entrypoint.sh /entrypoint

ENTRYPOINT ["/entrypoint"]
