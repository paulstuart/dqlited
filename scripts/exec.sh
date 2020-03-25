#!/bin/bash

# if we're already in docker than lets skip the docker exec

if [[ -f /.dockerenv ]]; then
   echo "running inside docker" >&2
   "$@"
   exit
fi
 
dockerish() {
    # the old way of testing in a single docker container
    RELEASE=${RELEASE:-xenial}
    MNT=/root/go/src/github.com/paulstuart/dqlited 
    #NAME=paulstuart/dqlited
    NAME=paulstuart/dqlite-dev
    IMG=${NAME}:${RELEASE}

    docker ps --format '{{.Image}} {{.Names}}'
    EXEC_ID=$(docker ps --format '{{.Image}} {{.Names}}' | \
	awk '$1~v { print $2 }' v=${NAME})

    docker exec -it -w $MNT $EXEC_ID "$@"
}


# TODO: make this work with compose or dq instance automagically
#dockerish $@; exit

# now we're running in docker-compose
docker-compose -p dqlited -f docker/docker-compose.yml exec bastion $@
exit

