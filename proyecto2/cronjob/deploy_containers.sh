#!/bin/bash
# deploy_containers.sh - despliega contenedores de prueba

IMAGES=(
  "roldyoran/go-client"
  "alpine sh -c \"while true; do echo '2^20' | bc > /dev/null; sleep 2; done\""
  "alpine sleep 240"
)

for i in 1 2 3 4 5; do
    IDX=$((RANDOM % 3))
    NAME="carga_$(date +%s)_$i"
    case $IDX in
        0) docker run -d --name "$NAME" roldyoran/go-client ;;
        1) docker run -d --name "$NAME" alpine sh -c "while true; do echo '2^20' | bc > /dev/null; sleep 2; done" ;;
        2) docker run -d --name "$NAME" alpine sleep 240 ;;
    esac
done

# Contenedor intruso, uno por ejecución
docker run -d --name "intruso_$(date +%s)" intruso-202112674:v1

echo "$(date): 5 contenedores + 1 intruso desplegados" >> /tmp/cronjob_pr2.log