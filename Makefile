.PHONY: docker run

MNT = /root/go/src/github.com/paulstuart/dqlited 

docker:
	docker build -t paulstuart/xenial-dqlite:latest .

run:
	docker run \
		-it --rm \
		-p 4001:4001 \
		--security-opt seccomp=unconfined \
		--workdir $(MNT) \
                --mount type=bind,src="$$PWD",dst=$(MNT) \
		paulstuart/xenial-dqlite:latest bash

