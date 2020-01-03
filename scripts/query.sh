#!/bin/bash

URL="http://localhost:4001/db/query"

QUERY=${*}
[[ -z $QUERY ]] && QUERY="select * from sqlite_master"
curl -s -G \
	--data-urlencode "q=$QUERY" \
	$URL | jq .results
