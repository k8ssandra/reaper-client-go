#!/bin/bash
#
# A wrapper script for adding a cluster to Reaper. It expected two args,
# the cluster name and a seed host.

set -e

if [ $# -ne 2 ]; then
  echo "Usage: add-cluster.sh <cluster> <seed>"
  exit 1
fi

cluster=$1
seed=$2

curl -H "Content-Type: application/json" -X PUT http://localhost:8080/cluster/$cluster?seedHost=$seed
