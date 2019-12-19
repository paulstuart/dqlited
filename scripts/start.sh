#!/bin/bash

# start in project root directory
cd $(dirname $0)/..
export PATH=$PWD:$PATH

# fail at first error
set -e

CMD=dqlited

start() {
   local id=$1
   #[[ $1 -gt 3 ]] && cluster="$(scripts/active.sh)" && cluster="-c $cluster"
   #echo "ID: $id CLUSTER: $cluster"
   $CMD start $cluster $id >> /tmp/dqlited-demo${id}.txt 2>&1 &
   # don't need to add first server to itself
   [[ $1 == 1 ]] && sleep 1 && return
   sleep 2
   $CMD add $id
}

#
# start with 3 nodes for a minimum Raft consensus
#
default() {
    # start fresh 
    rm -rf /tmp/dqlite*

    for id in {1..3}; do
       start $id
    done
}

if [[ $# -eq 0 ]]; then
    default
    exit
fi

for id in $@; do
   start $id
done

