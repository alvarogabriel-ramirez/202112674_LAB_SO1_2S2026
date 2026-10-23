#!/bin/bash
# Construye la imagen de Grafana con el plugin y la publica en Zot.
set -e
REG=zot.local:5000/mumnk8s
cd "$(dirname "$0")/.."
docker build -t $REG/grafana-valkey:11.3.0 grafana-image/
docker push $REG/grafana-valkey:11.3.0
curl -sk https://zot.local:5000/v2/_catalog
