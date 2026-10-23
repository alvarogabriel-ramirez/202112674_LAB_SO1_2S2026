#!/bin/bash
IP=$(docker network inspect kind | jq -r '.[0].IPAM.Config[] | select(.Gateway | contains(".")) | .Gateway')
for n in $(kind get nodes --name mumnk8s); do
  docker exec $n sh -c "grep -q zot.local /etc/hosts || echo '$IP zot.local' >> /etc/hosts"
done
echo "zot.local -> $IP"