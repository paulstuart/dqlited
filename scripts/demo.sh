#!/bin/bash

# start in parent directory
cd $(dirname $0)/..
export PATH=$PWD:$PATH

# start fresh
rm -rf /tmp/dqlite*

# fail at first error
set -e

CMD=dqlited

$CMD start 1 > /tmp/dqlited-demo1.txt 2>&1 &
#sleep 1
$CMD start 2 > /tmp/dqlited-demo2.txt 2>&1 &
#sleep 1
$CMD start 3 > /tmp/dqlited-demo3.txt 2>&1 &
#sleep 1
$CMD add 2
#sleep 1
$CMD add 3
