FROM alpine:3.6

MAINTAINER Minio Inc <dev@minio.io>

ENV GOPATH /go
ENV PATH $PATH:$GOPATH/bin
ENV CGO_ENABLED 0

WORKDIR /go/src/github.com/minio/

RUN  \
     apk add --no-cache ca-certificates && \
     apk add --no-cache --virtual .build-deps git go musl-dev && \
     echo 'hosts: files mdns4_minimal [NOTFOUND=return] dns mdns4' >> /etc/nsswitch.conf && \
     go get -v -d github.com/pydio/minio-srv && \
     cd /go/src/github.com/pydio/minio-srv && \
     go install -v -ldflags "$(go run buildscripts/gen-ldflags.go)" && \
     rm -rf /go/pkg /go/src /usr/local/go && apk del .build-deps

EXPOSE 9000

COPY buildscripts/docker-entrypoint.sh /usr/bin/

RUN chmod +x /usr/bin/docker-entrypoint.sh

ENTRYPOINT ["/usr/bin/docker-entrypoint.sh"]

VOLUME ["/export"]

CMD ["minio"]
