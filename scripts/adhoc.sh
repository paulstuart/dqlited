#!/bin/bash

cd $(dirname $0)
./exec.sh ./dqlited adhoc "$*"
