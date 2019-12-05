.PHONY: docker run build

MNT = /root/go/src/github.com/paulstuart/dqlited 

build:
	go build -tags libsqlite3

docker:
	docker build -t paulstuart/xenial-dqlite:latest .

run:
	docker run \
		-it --rm \
		-p 4001:4001 \
		--workdir $(MNT) \
                --mount type=bind,src="$$PWD",dst=$(MNT) \
		paulstuart/xenial-dqlite:latest bash

