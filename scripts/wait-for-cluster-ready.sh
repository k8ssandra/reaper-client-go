#!/bin/bash
#
# This is a poor man's readiness probe for a cluster that is run via
# docker-compose. The script expects as input the name of one of the C* nodes
# as specified in docker-compose.yml. It then checks that each of the nodes is
# reporting UN for its status. 
# 
# Note that this script assumes that the cluster has two nodes.

set -e

if [ $# -ne 1 ]; then
  echo "Usage: wait-for-cluster-ready.sh <cassandra-service>"
  exit 1
fi

node=$1

get_nodes_ready() {
  docker-compose exec $node nodetool -u reaperUser -pw reaperPass status | grep UN | wc -l
}

limit=4
count=0

until [ $count -gt $limit ];
do
  if [ `get_nodes_ready` -eq 2 ]; then
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
