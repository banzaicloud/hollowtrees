# build stage
FROM golang:1.9.3-alpine3.7

ADD . /go/src/github.com/banzaicloud/hollowtrees
WORKDIR /go/src/github.com/banzaicloud/hollowtrees
RUN go build -o /bin/hollowtrees .

FROM alpine:latest
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
COPY --from=0 /bin/hollowtrees /bin
ADD ./conf/config.yaml /root/conf/
ENTRYPOINT ["/bin/hollowtrees"]
