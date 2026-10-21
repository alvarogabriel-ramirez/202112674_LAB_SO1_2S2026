# Guía de Instalación — Proyecto 2 SO1
### Sonda de Kernel en C y Daemon en Go para la Telemetría de Contenedores

Esta guía asume Ubuntu limpio. Reemplaza **202112674** por tu carnet y **gabriel** por tu usuario real del sistema en todas las rutas.

---

## 1. Paquetes base

```bash
sudo apt update
sudo apt install -y build-essential linux-headers-$(uname -r) git curl wget \
  clang llvm libelf-dev libbpf-dev bpftool linux-tools-common linux-tools-$(uname -r) \
  redis-tools jq
```

## 2. Docker y Docker Compose

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
newgrp docker
sudo apt install -y docker-compose-plugin
docker --version
docker compose version
```

## 3. Go y herramientas de eBPF

```bash
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc
go version
go install github.com/cilium/ebpf/cmd/bpf2go@latest
```

## 4. Clonar el repositorio

```bash
git clone https://github.com/TU_USUARIO/202112674_LAB_SO1_2S2026.git
cd 202112674_LAB_SO1_2S2026/proyecto2
```

## 5. Módulo de kernel

```bash
cd kernel_module
make
sudo insmod continfo.ko
lsmod | grep continfo
cat /proc/continfo_pr2_so1_202112674
```

Si necesitas recompilar/recargar:
```bash
chmod +x load_module.sh
./load_module.sh
```

## 6. Imágenes Docker

```bash
docker pull roldyoran/go-client
docker pull alpine

cd ../docker_images/intruso
docker build -t intruso-202112674:v1 .
```

## 7. Script de cronjob (prueba manual antes de que el Daemon lo automatice)

```bash
cd ../../cronjob
chmod +x deploy_containers.sh
./deploy_containers.sh
docker ps
```

## 8. Generar el binding del programa eBPF

```bash
cd ../ebpf
bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h
```

Copia `killtrace.c` y `vmlinux.h` a la carpeta `daemon/` (o ajusta las rutas del `//go:generate`).

## 9. Compilar el Daemon

```bash
cd ../daemon
go mod init daemon
go get github.com/cilium/ebpf github.com/cilium/ebpf/link github.com/redis/go-redis/v9
go generate ./...
go mod tidy
go build -o daemon .
```

> Antes de compilar, edita en `main.go` las constantes `cronScript` y `loadModuleSh` para que apunten a tu ruta real, por ejemplo:
> ```go
> cronScript   = "/home/gabriel/proyecto2/cronjob/deploy_containers.sh"
> loadModuleSh = "/home/gabriel/proyecto2/kernel_module/load_module.sh"
> ```

## 10. docker-compose.yml (Valkey + Grafana)

Ya debe estar en la raíz de `proyecto2/`. Levántalo antes de correr el Daemon la primera vez, para probarlo por separado:

```bash
cd ..
docker compose up -d
docker ps
redis-cli -h localhost -p 6379 ping    # debe responder PONG
```

## 11. Configurar Grafana

```bash
docker exec -it grafana grafana-cli plugins install redis-datasource
docker restart grafana
```

Entra a `http://localhost:3000` (admin/admin) → **Connections → Data sources → Add data source → Redis** → Address: `valkey:6379` → **Save & Test**.

## 12. Importar el dashboard

**Dashboards → New → Import** → sube `grafana/dashboard.json` → selecciona el datasource Redis → **Import**.

## 13. Correr el Daemon (esto arranca todo automáticamente)

```bash
cd daemon
sudo ./daemon
```

Déjalo corriendo en una terminal visible. Va a:
1. Levantar Grafana + Valkey (si no estaban ya arriba).
2. Instalar el cronjob.
3. Compilar y cargar el módulo de kernel.
4. Adjuntar el programa eBPF.
5. Empezar el loop de gestión cada 30 segundos.

## 14. Verificación completa

```bash
lsmod | grep continfo
cat /proc/continfo_pr2_so1_202112674 | head -20
docker ps
crontab -l
ps aux | grep daemon
redis-cli -h localhost -p 6379 KEYS '*'
redis-cli -h localhost -p 6379 GET ultima_ram_total
redis-cli -h localhost -p 6379 HGETALL top5_ram
```

Abre `http://localhost:3000` y confirma que el dashboard muestra datos.

---

