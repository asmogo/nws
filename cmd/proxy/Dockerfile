FROM golang:1.21-alpine as builder

ADD . /build/

WORKDIR /build
RUN apk add --no-cache git bash openssh-client && \
    go build -o proxy cmd/proxy/*.go


#building finished. Now extracting single bin in second stage.
FROM alpine

COPY --from=builder /build/proxy /app/

WORKDIR /app

CMD ["./proxy"]