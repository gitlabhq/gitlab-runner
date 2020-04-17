FROM golang:1.13.8

# Install required dependencies
RUN apt-get update -yq && \
    apt-get install -yq locales make xz-utils \
                        ruby ruby-dev python-pip \
                        dpkg-sig createrepo rpm \
                        zip libffi-dev jq && \
    rm -rf /var/lib/apt/lists/*

# Set default locale for the environment
RUN echo "en_US UTF-8" > /etc/locale.gen; \
    locale-gen en_US.UTF-8

ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8

ARG DOCKER_VERSION

# Install docker client
RUN wget -q https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz -O /tmp/docker.tar.gz; \
    tar -xzf /tmp/docker.tar.gz -C /tmp/; \
    cp /tmp/docker/docker* /usr/bin; \
    chmod +x /usr/bin/docker*; \
    rm -rf /tmp/*

COPY Makefile* /tmp/
WORKDIR /tmp
RUN make deps package-deps packagecloud-deps

RUN go get -u github.com/jstemmer/go-junit-report
