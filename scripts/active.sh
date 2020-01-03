#!/bin/bash

# show dqlited listening ports
# pass -w as an argument to watch while active

let delay=1
let total=0

# all ports of interest, e.g., 918x (where 1 through 5)
active() { netstat -aln | awk '$6 ~ /LISTEN|TIME_WAIT/ && $4 ~ /:918[1-5]/ {print $4}' | sort -u ; }

[[ -z $1 ]] && active && exit

while true
do
  LISTENING=$(active | paste -d, -s - )
  [[ -z $LISTENING ]]   && exit
  [[ $total -eq 0 ]] && echo
  printf "$(date) (waiting ${total}s) -- waiting for: ${LISTENING}\r"
  [[ $1 == "-w" ]]   || break
  sleep $delay
  let total+=$delay
done

# finish off with  a new line for a clean break
printf "\033[K" # clear to end of line
echo
