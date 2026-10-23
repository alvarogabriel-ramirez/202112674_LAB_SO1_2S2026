#!/bin/bash
# Construye y publica todas las imagenes en Zot. Uso: ./scripts/build-push.sh [tag]
set -e
REG=zot.local:5000/mumnk8s
TAG=${1:-v1}
cd "$(dirname "$0")/.."

echo ">> Espejo de RabbitMQ en Zot"
docker pull rabbitmq:3.13-management
docker tag rabbitmq:3.13-management $REG/rabbitmq:3.13-management
docker push $REG/rabbitmq:3.13-management

for svc in rest-api grpc-client grpc-server rabbit-writer; do
  echo ">> go-$svc"
  docker build -t $REG/go-$svc:$TAG --build-arg SERVICE=$svc go/
  docker push $REG/go-$svc:$TAG
done

echo ">> rust-api"
docker build -t $REG/rust-api:$TAG rust-api/
docker push $REG/rust-api:$TAG

echo ">> Listo. Catalogo:"
curl -sk https://zot.local:5000/v2/_catalog
