#!/bin/bash
#
# This is a poor man's readiness probe for a cluster that is run via
# docker-compose. The script expects as input the name of one of the C* nodes
# as specified in docker-compose.yml and the number of nodes that should be
# ready. It then checks that at least the specified number of nodes is
# reporting UN for its status. 
# 

set -e

if [ $# -ne 2 ]; then
  echo "Usage: wait-for-cluster-ready.sh <cassandra-service> <num-nodes>"
  exit 1
fi

node=$1
num_nodes=$2

get_nodes_ready() {
  docker-compose exec $node nodetool -u reaperUser -pw reaperPass status | grep UN | wc -l
}

limit=4
count=0

until [ $count -gt $limit ];
do
  if [ `get_nodes_ready` -eq $num_nodes ]; then
    break
  else
    ((count++))
    echo "waiting for cluster with node $node to be ready"
    sleep 1
  fi
done

if [ $count -ge $limit ]; then
  echo "timed out waiting for cluster with node $node to be ready"
  exit 1
else
  echo "cluster with node $node is ready" 
fi
