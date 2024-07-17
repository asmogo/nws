FROM golang:1.21-alpine as builder

ARG DEPLOY_KEY

ADD . /build/

WORKDIR /build
RUN apk add --no-cache git bash openssh-client && \
    go build -o exit cmd/exit/*.go


#building finished. Now extracting single bin in second stage.
FROM alpine

COPY --from=builder /build/exit /app/

WORKDIR /app

CMD ["./exit"]