ARG release=xenial

FROM paulstuart/dqlite-dev:${release} as builder

WORKDIR /root/go/src/github.com/paulstuart/dqlited/

RUN make static

FROM ubuntu:${release}

RUN apt-get update

COPY  --from=builder /root/go/src/github.com/paulstuart/dqlited/dqlited /usr/local/bin/

COPY  --from=builder /usr/local/bin/sqlite3 /usr/local/bin/


