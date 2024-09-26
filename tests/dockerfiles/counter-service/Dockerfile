FROM alpine

RUN apk update && apk add bash

CMD ["bash", "-c", "counter=1; while [ $counter -le 10 ]; do echo $counter; ((counter++)); sleep 1; done"]
