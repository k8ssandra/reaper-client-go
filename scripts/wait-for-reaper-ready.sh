#!/bin/bash
#
# This is a readiness probe for Reaper that is run via docker-compose. 
# 
# Note that this script requires jq to be installed.

set -e


get_reaper_ready() {
  curl -s http://localhost:8081/healthcheck | jq .reaper.healthy
}

limit=4
count=0

until [ $count -gt $limit ];
do
  if [ `get_reaper_ready` == "true" ]; then
    break
  else
    ((count++))
    echo "waiting for reaper to be ready"
    sleep 1
  fi
done

if [ $count -ge $limit ]; then
  echo "timed out waiting for reaper to be ready"
  exit 1
else
  echo "reaper is ready" 
fi
