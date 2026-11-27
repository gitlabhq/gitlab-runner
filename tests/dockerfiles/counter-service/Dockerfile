FROM alpine:3.24

# Alpine removes superseded package versions, so pinned versions break the build
# hadolint ignore=DL3018
RUN apk add --no-cache bash

CMD ["bash", "-c", "counter=1; while [ $counter -le 10 ]; do echo $counter; ((counter++)); sleep 1; done"]
