FROM golang:1.15-alpine

RUN apk add git

WORKDIR /go/src/github.com/fako1024/cmdchat/server
COPY server.go .

RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -o cmdchat-server .

#########################

FROM alpine:3.12

WORKDIR /
COPY --from=0 /go/src/github.com/fako1024/cmdchat/server/cmdchat-server .

RUN addgroup -g 10000 -S app && \
    adduser -u 10000 -S app -G app

USER app

CMD ["/cmdchat-server"]
