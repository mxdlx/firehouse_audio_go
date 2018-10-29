FROM golang:1.10-alpine3.7

WORKDIR /go/src/app
COPY service.go .

RUN apk add --no-cache git vlc
RUN go get -d -v ./...
RUN go install -v ./...

RUN adduser -h /home/vlc -D -s /bin/ash vlc
RUN mkdir /logs /firehouse
RUN chown vlc:vlc /logs /firehouse

CMD ["app"]
