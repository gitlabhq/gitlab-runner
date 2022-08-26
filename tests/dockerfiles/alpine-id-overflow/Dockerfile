FROM alpine:3.14.2

RUN rm /usr/bin/id && \
    printf "#!/bin/sh -e\ncat /dev/zero\n" > /bin/id && \
    chmod +x /bin/id

# non-root user required to enable the docker executor's file ownership change
USER 1000:1000

ENTRYPOINT ["/bin/sh", "-c"]
