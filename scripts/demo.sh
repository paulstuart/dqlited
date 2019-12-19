#!/bin/bash

cd $(dirname $0)

pkill dqlited

./start.sh 	# start 3 (required) servers

sleep 1		# let it warm up

./prep.sh 	# add some schemas/data

./start.sh 4 5 	# start extra servers for failover testing
