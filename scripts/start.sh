#!/bin/bash

# this script is intended for use in a development context,
# for running muliple servers locally for test purposes

# allow core dumps
ulimit -c unlimited
echo "core.%e.%p" > /proc/sys/kernel/core_pattern

#export GOTRACEBACK=crash

# start in project root directory
cd $(dirname $0)/..
export PATH=$PWD:$PATH

# fail at first error
set -e

[[ -n $DEBUG ]] && echo >&2 DQLITED_CLUSTER=$DQLITED_CLUSTER

CMD=dqlited
DIR=/tmp/dqlited
PIDS=$DIR/pids

# get timestamp matching Go's formatting
ts() { date +"%Y/%m/%d %H:%M:%S.%N" |  cut -c -26; }

# get status of port listener
portstat() { netstat -tln | awk -v port="$1" '$4 ~ ":"port"$" {print $4}'; }

portwait() { 
    PORT=$1
    while [[ -n $(portstat $PORT) ]]
    do
	    echo "waiting for close: $PORT"
	    sleep 1
    done
}

[[ -d $PIDS ]] || mkdir -p  $PIDS

server_start() {
   DQLITED_ID=$1
   rm -rf $DIR/$DQLITED_ID > /dev/null 2>&1
   echo "$(ts) starting server ${DQLITED_ID}"  							>> /tmp/dqlited-demo${DQLITED_ID}.txt 
#   while nc -z localhost $PORT; do   
#	echo "$(ts) waiting for close of port $PORT" >> /tmp/dqlited-demo${DQLITED_ID}.txt
#   	sleep 1
#   done
   $CMD server --id=${DQLITED_ID} --address=127.0.0.1:918${DQLITED_ID} --port=400${DQLITED_ID}	>> /tmp/dqlited-demo${DQLITED_ID}.txt 2>&1 &
   echo $! > $PIDS/$DQLITED_ID
   sleep 1 # let it start up before doing anything with it
   echo "$(ts) node:$DQLITED_ID started with pid:$!" 						>> /tmp/dqlited-demo${DQLITED_ID}.txt
}

server_stop() {
   DQLITED_ID=$1
   PID=$(cat $PIDS/$DQLITED_ID)
   echo "$(ts) kill server: $DQLITED_ID with pid: $PID" >> /tmp/dqlited-demo${DQLITED_ID}.txt
   kill -SIGINT $PID && echo "killed pid $PID"
   # wait for process to terminate. TODO: use a timeout?
   tail --pid=$PID -f /dev/null
   PORT=918${DQLITED_ID}
   echo "$(ts) checking open port: $PORT" >> /tmp/dqlited-demo${DQLITED_ID}.txt
   portwait $PORT
#   while nc -z localhost $PORT; do   
#	   echo "$(ts) waiting for close of port $PORT" >> /tmp/dqlited-demo${DQLITED_ID}.txt
#   sleep 1
#   done
   echo "$(ts) cleaning pid file" >> /tmp/dqlited-demo${DQLITED_ID}.txt
   rm -f $PIDS/$DQLITED_ID
   sleep 1 # still seeing bind errors, so wait a tiny bit more
}


#
# 3 nodes is the minimum Raft consensus
#

default() {
    # start fresh 
    rm -rf $DIR/dqlite*

    # the first node doesn't need to be added to itself
    export DQLITED_SKIP="true"
    for id in {1..3}; do
       server_start $id
       export DQLITED_SKIP="false"
    done
    exit
}

[[ $# -eq 0 ]] && default

# restart server(s)?
if [[ $1 == "-r" ]]; then
	shift
	for id in $@; do
	   server_stop  $id
	   server_start $id
	done
	exit
fi

# start server(s)
for id in $@; do
   server_start $id
done

