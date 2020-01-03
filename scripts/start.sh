#!/bin/bash

# allow core dumps
ulimit -c unlimited
echo "core.%e.%p" > /proc/sys/kernel/core_pattern


# start in project root directory
cd $(dirname $0)/..
export PATH=$PWD:$PATH

# fail at first error
set -e

echo DQLITED_CLUSTER=$DQLITED_CLUSTER

CMD=dqlited

start() {
   DQLITED_ID=$1
   rm -rf /tmp/dqlited/$DQLITED_ID > /dev/null 2>&1
   $CMD server --id=${DQLITED_ID} --address=127.0.0.1:918${DQLITED_ID} --port=400${DQLITED_ID} >> /tmp/dqlited-demo${DQLITED_ID}.txt 2>&1 &
   sleep 1
}

#
# 3 nodes is the minimum Raft consensus
#
default() {
    # start fresh 
    rm -rf /tmp/dqlite*

    export DQLITED_SKIP="true"
    for id in {1..3}; do
       start $id
       export DQLITED_SKIP="false"
    done
    exit
}

[[ $# -eq 0 ]] && default

for id in $@; do
   start $id
done

