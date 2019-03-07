FROM alpine:3.9

RUN apk add --no-cache bash ca-certificates git git-lfs miniperl \
	&& ln -s miniperl /usr/bin/perl

RUN git lfs install --skip-repo

COPY ./scripts/ /usr/bin
COPY ./binaries/gitlab-runner-helper.x86_64 /usr/bin/gitlab-runner-helper

RUN echo 'hosts: files dns' >> /etc/nsswitch.conf
