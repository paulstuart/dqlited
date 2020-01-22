#!/bin/bash

bye() { echo >&2 $@; exit 1; }

ID=$1

[[ $# -eq 0  ]] || bye "no node ID specified"
[[ $ID -gt 0 ]] || bye "ID must be greater than 0"

PID=$(cat /tmp/dqlited/pids/$ID)
[[ -n $PID ]] || bye "no PID for leader $ID"
ps -p $PID | grep -q dqlited || bye "PID $PID is not a dqlite process id"
kill -SIGINT $PID
