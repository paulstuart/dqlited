#!/bin/bash

# show dqlited listening ports
# pass -w as an argument to watch while active

let delay=5
let total=0


while true
do
  active=$(netstat -aln | awk '$4 ~ /127.0.0.1:91/ {print $4}' | sort -u | paste -d, -s - )
  [[ -z $active ]] && exit
  [[ $total -eq 0 ]] && echo
  printf "$(date) (waiting ${total}s) -- waiting for: ${active}\r"
  [[ $1 -ne "-w" ]] && break
  sleep $delay
  let total+=$delay
done

# finish off with  a new line for a clean break
echo
