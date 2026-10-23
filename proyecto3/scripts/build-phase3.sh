#!/bin/bash
# Publica en Zot: consumer (Go) y espejo de Valkey. Uso: ./scripts/build-phase3.sh [tag]
set -e
REG=zot.local:5000/mumnk8s
TAG=${1:-v1}
cd "$(dirname "$0")/.."

docker pull valkey/valkey:8
docker tag valkey/valkey:8 $REG/valkey:8
docker push $REG/valkey:8

docker build -t $REG/go-consumer:$TAG --build-arg SERVICE=consumer go/
docker push $REG/go-consumer:$TAG

curl -sk https://zot.local:5000/v2/_catalog
