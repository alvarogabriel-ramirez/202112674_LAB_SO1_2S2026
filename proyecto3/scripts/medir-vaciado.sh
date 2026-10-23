#!/bin/bash
# Uso: ./scripts/medir-vaciado.sh <replicas_del_consumer>
# Enciende el consumer con N réplicas y mide cuánto tarda la cola war_reports en llegar a 0.
REP=${1:-1}
q() { kubectl exec -n mumnk8s deploy/rabbitmq -- rabbitmqctl list_queues name messages --quiet 2>/dev/null | awk '$1=="war_reports"{print $2}'; }
INICIAL=$(q)
echo "Mensajes en cola al inicio: $INICIAL"
start=$(date +%s)
kubectl scale deploy/go-consumer -n mumnk8s --replicas=$REP
while true; do
  n=$(q); echo "$(( $(date +%s)-start )) s   mensajes=$n"
  [ "$n" = "0" ] && break
  sleep 3
done
T=$(( $(date +%s)-start ))
echo "RESULTADO: $REP réplica(s) | inicio=$INICIAL mensajes | tiempo=$T s"
