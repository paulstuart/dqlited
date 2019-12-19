#!/bin/bash

# if we're already in docker than lets skip the docker exec

if [[ -f /.dockerenv ]]; then
   "$@"
   exit
fi
 
RELEASE=xenial
MNT=/root/go/src/github.com/paulstuart/dqlited 
NAME=paulstuart/dqlited
IMG=${NAME}:${RELEASE}

EXEC_ID=$(docker ps --format '{{.Image}} {{.Names}}' | \
	awk '$1~v { print $2 }' v=${NAME}) 

docker exec -it -w $MNT $EXEC_ID "$@"

