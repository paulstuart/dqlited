#!/bin/bash

# if we're already in docker than lets skip the docker exec

if [[ -f /.dockerenv ]]; then
   "$@"
   exit
fi
 
# now we're running in docker-compose
docker-compose -p dqlited -f docker/docker-compose.yml exec bastion $@
exit

# the old way of testing in a single docker container
RELEASE=xenial
MNT=/root/go/src/github.com/paulstuart/dqlited 
NAME=paulstuart/dqlited
IMG=${NAME}:${RELEASE}

EXEC_ID=$(docker ps --format '{{.Image}} {{.Names}}' | \
	awk '$1~v { print $2 }' v=${NAME}) 

docker exec -it -w $MNT $EXEC_ID "$@"

