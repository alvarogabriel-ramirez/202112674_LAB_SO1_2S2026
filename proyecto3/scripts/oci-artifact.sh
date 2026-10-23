#!/bin/bash
# Publica (push) o descarga (pull) el archivo de entrada de Locust como OCI Artifact en Zot.
# Uso: ./scripts/oci-artifact.sh push | pull
set -e
REF=zot.local:5000/mumnk8s/locust-input:v1
CA=$HOME/zot/certs/zot.crt
cd "$(dirname "$0")/.."
case "$1" in
  push)
    cd locust
    oras push $REF --ca-file $CA \
      --artifact-type application/vnd.mumnk8s.locust-input.v1+json \
      locust-input.json:application/json
    ;;
  pull)
    oras pull $REF --ca-file $CA -o locust/input
    ls -l locust/input
    ;;
  *) echo "Uso: $0 push|pull"; exit 1 ;;
esac
