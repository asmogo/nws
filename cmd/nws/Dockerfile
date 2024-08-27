FROM golang:1.21-alpine as builder

ADD . /build/

WORKDIR /build
RUN apk add --no-cache git bash openssh-client && \
    go build -o nws cmd/nws/*.go


#building finished. Now extracting single bin in second stage.
FROM alpine

COPY --from=builder /build/nws /app/

WORKDIR /app

CMD ["./nws"]