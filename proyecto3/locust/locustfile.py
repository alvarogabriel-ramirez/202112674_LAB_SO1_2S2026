import json
import os
import random
from datetime import datetime, timezone

from locust import HttpUser, task, between

# Archivo de entrada descargado de Zot como OCI Artifact.
# Se obtiene con: ./scripts/oci-artifact.sh pull
INPUT = os.path.join(os.path.dirname(__file__), "input", "locust-input.json")
if not os.path.exists(INPUT):
    raise SystemExit("Falta locust/input/locust-input.json. Ejecuta ./scripts/oci-artifact.sh pull")
with open(INPUT) as f:
    CFG = json.load(f)


class ReporteMilitar(HttpUser):
    wait_time = between(0.01, 0.1)

    @task
    def enviar_reporte(self):
        self.client.post(CFG["route"], json={
            "country": random.choice(CFG["countries"]),
            "warplanes_in_air": random.randint(CFG["warplanes_in_air"]["min"], CFG["warplanes_in_air"]["max"]),
            "warships_in_water": random.randint(CFG["warships_in_water"]["min"], CFG["warships_in_water"]["max"]),
            "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        })
