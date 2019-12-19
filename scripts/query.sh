#!/bin/bash

URL="http://localhost:4001/db/query"

curl -s -G \
	--data-urlencode "q=select * from model" \
	$URL | jq .results
