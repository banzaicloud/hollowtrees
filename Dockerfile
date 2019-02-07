ARG GO_VERSION=1.11

FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --update --no-cache ca-certificates=20190108-r0 make=4.2.1-r2 git=2.20.1-r0 curl=7.63.0-r0

ARG PACKAGE=github.com/banzaicloud/hollowtrees

RUN mkdir -p /go/src/${PACKAGE}
WORKDIR /go/src/${PACKAGE}

COPY Gopkg.* Makefile /go/src/${PACKAGE}/
RUN make vendor

COPY . /go/src/${PACKAGE}
RUN BUILD_DIR='' BINARY_NAME=app make build-release


FROM alpine:3.7
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app /app
USER nobody:nobody
CMD ["/app"]
