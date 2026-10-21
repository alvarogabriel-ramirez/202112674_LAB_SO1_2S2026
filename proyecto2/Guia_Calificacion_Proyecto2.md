# Checklist de Encendido + Guía de Calificación — Proyecto 2 SO1

Carnet: **202112674** — Usa este documento el día de la calificación, en orden.

---

## PARTE 1 — Encender todo antes de la llamada

### 1. Verifica permisos de Docker

```bash
groups
```
Si no ves `docker` en la lista:
```bash
sudo usermod -aG docker $USER
newgrp docker
```

### 2. Limpia contenedores viejos de pruebas anteriores (opcional pero recomendable)

```bash
docker ps -a
docker ps -a --format '{{.Names}}' | grep -E '^carga_|^intruso_' | xargs -r docker rm -f
```

### 3. Descarga el módulo de kernel si estaba cargado de una sesión anterior

```bash
lsmod | grep continfo
sudo rmmod continfo 2>/dev/null
```

### 4. Verifica que no haya un Daemon viejo corriendo de fondo

```bash
ps aux | grep daemon
# si aparece uno viejo, mátalo
sudo pkill -f "./daemon"
```

### 5. Limpia el cronjob viejo (el Daemon lo va a reinstalar al arrancar)

```bash
crontab -l
crontab -l | grep -v "deploy_containers.sh" | crontab -
```

### 6. Levanta Grafana + Valkey manualmente primero

```bash
cd ~/proyecto2
docker compose up -d
docker ps
```
Confirma que ambos digan `Up`:
```bash
redis-cli -h localhost -p 6379 ping         # debe decir PONG
curl -s http://localhost:3000/api/health    # debe responder algo como {"database":"ok",...}
```

### 7. Arranca el Daemon (esto dispara todo lo demás: cronjob, módulo, eBPF)

```bash
cd ~/proyecto2/daemon
sudo ./daemon
```
Déjalo en una terminal aparte, visible — el auxiliar puede pedirte ver su salida en vivo (especialmente en la Vuelta 4 de eBPF).

### 8. Deja pasar al menos un ciclo completo antes de la llamada

El Daemon corre su loop cada 20-60s y el cronjob cada 2 minutos — **espera unos 3-5 minutos** con todo corriendo antes de la calificación, para que:
- Ya haya snapshots en Valkey (`redis-cli KEYS 'snapshot:*'`).
- El cronjob ya haya desplegado al menos una tanda de contenedores.
- El Daemon ya haya aplicado la lógica de mínimos (3 bajo consumo, 2 alto consumo).

### 9. Ten listo (sin ejecutar todavía) el comando para la Vuelta 4

```bash
docker ps --format '{{.Names}}' | grep -E '^carga_|^intruso_' | head -1
```
Y ten lista la ventana del Daemon para mostrar el log `[eBPF] emisor=... objetivo=... señal=...` en tiempo real cuando te lo pidan.

**Resumen del orden correcto:** permisos Docker → limpiar restos viejos → `docker compose up -d` → arrancar el Daemon con `sudo` → esperar unos minutos → tener Grafana abierto en el navegador.

---

## PARTE 2 — Guía de Calificación (comandos exactos por vuelta)

> **REQUISITO:** tener todo levantado — módulo de kernel, Daemon, cronjob, Grafana, Valkey — antes de empezar.

### Vuelta 1 — Módulo de Kernel

```bash
lsmod | grep continfo
cat /proc/continfo_pr2_so1_202112674
```
Se verificará: el módulo aparece en `lsmod`, el archivo existe y muestra datos legibles (RAM total/libre/uso + lista de procesos con PID, nombre, VSZ, RSS, %MEM, %CPU).

### Vuelta 2 — Extracción de datos correcta + Cronjob y restricciones de contenedores

Mismo archivo de la vuelta 1, pero ahora se revisa el contenido a detalle:
- ¿Los PID corresponden a contenedores reales corriendo? (comparar con `docker ps`)
- ¿Los valores de memoria/CPU tienen sentido?

```bash
docker ps --format "table {{.ID}}\t{{.Names}}\t{{.Status}}"
crontab -l
docker ps
```

### Vuelta 3 — Daemon GO + Valkey

```bash
ps aux | grep daemon
redis-cli -h localhost -p 6379 KEYS '*'
redis-cli -h localhost -p 6379 GET ultima_ram_total
redis-cli -h localhost -p 6379 HGETALL top5_ram
```
Se verificará: el daemon está corriendo (proceso vivo), y Valkey tiene registros con datos parseados (no vacío, no basura).

### Vuelta 4 — eBPF

Se te pedirá matar un contenedor y enseñar en vivo:
- El **log/salida** del programa eBPF capturando el evento (PID emisor, PID receptor, señal).
- El registro correspondiente en Valkey confirmando "contenedor eliminado" **solo después** de la captura eBPF.

```bash
docker ps --format '{{.Names}}' | grep -E '^carga_|^intruso_' | head -1
docker stop <nombre_copiado>
```
En la terminal del Daemon debe aparecer:
```
[eBPF] emisor=... objetivo=... señal=...
```


### Vuelta 5 — Grafana

Se revisará el Dashboard: tarjetas de Total/Free RAM, contenedores eliminados en el tiempo, top 5 RAM/CPU, contador de eventos eBPF.

Ábrelo con anticipación en el navegador: `http://localhost:3000`

### Vuelta 6 — Documentación + pregunta(s) + Rust

Ten a la mano:
- El manual técnico (`MANUAL_TECNICO.md` / `.docx`).
- La guía de instalación.
- Respuestas preparadas sobre las decisiones técnicas (namespace de PID para identificar contenedores, tracepoint vs kprobe en eBPF, por qué Grafana/Valkey se excluyen de la gestión automática, etc.).
- Si hiciste la actividad extra de Rust, ten lista la demostración de `dmesg` mostrando `Hola Mundo 202112674`.

---

