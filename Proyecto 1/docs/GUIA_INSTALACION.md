# Guía de Instalación 



IPs usadas en esta implementación:
- VM1 (containerd): `192.168.122.4`
- VM2 (Podman): `192.168.122.44`
- VM3 (Docker + Zot): `192.168.122.115`

---

## 1. Preparar el host: KVM/QEMU

```bash
egrep -c '(vmx|svm)' /proc/cpuinfo      # debe ser > 0
sudo apt update
sudo apt install -y cpu-checker
kvm-ok

sudo apt install -y qemu-kvm libvirt-daemon-system libvirt-clients bridge-utils virtinst virt-manager
sudo usermod -aG libvirt,kvm $USER
# cierra sesión y vuelve a entrar
sudo systemctl status libvirtd          # debe decir "active (running)"
```

## 2. Descargar el sistema operativo de las VMs

```bash
mkdir -p ~/iso && cd ~/iso
wget https://releases.ubuntu.com/22.04/ubuntu-22.04.5-live-server-amd64.iso
```

## 3. Crear las 3 VMs 

Para cada una de las 3 VMs (`vm1-containerd`, `vm2-podman`, `vm3-docker`):

1. Abre **Virtual Machine Manager** → **Create a new virtual machine**.
2. **Local install media** → selecciona la ISO descargada.
3. RAM: **2048 MB**, CPUs: **2**.
4. Disco: **15 GB**.
5. Nombre de la VM → **Begin Installation**.
6. Durante el instalador de Ubuntu Server: tipo **Ubuntu Server**, red por DHCP, disco completo, crea tu usuario, y **activa "Install OpenSSH server"**.
7. Al terminar, anota la IP con `ip a` dentro de cada VM.

## 4. VM1 — containerd + Go

```bash
sudo apt update
sudo apt install -y containerd
sudo systemctl enable --now containerd

wget https://github.com/containerd/nerdctl/releases/download/v1.7.6/nerdctl-1.7.6-linux-amd64.tar.gz
sudo tar Cxzvvf /usr/local/bin nerdctl-1.7.6-linux-amd64.tar.gz

sudo mkdir -p /opt/cni/bin
wget https://github.com/containernetworking/plugins/releases/download/v1.5.1/cni-plugins-linux-amd64-v1.5.1.tgz
sudo tar Cxzvvf /opt/cni/bin cni-plugins-linux-amd64-v1.5.1.tgz

wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

## 5. VM2 — Podman + Go

```bash
sudo apt update
sudo apt install -y podman

wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

## 6. VM3 — Docker

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
# cierra sesión y vuelve a entrar
```

## 7. VM3 — Levantar el registro Zot

```bash
mkdir -p ~/zot && cd ~/zot
cat > config.json << 'EOF'
{
  "distSpecVersion": "1.1.0",
  "storage": { "rootDirectory": "/var/lib/registry" },
  "http": { "address": "0.0.0.0", "port": "5000" },
  "log": { "level": "debug" }
}
EOF

docker run -d -p 5000:5000 \
  -v $(pwd)/config.json:/etc/zot/config.json \
  -v zot-data:/var/lib/registry \
  --restart unless-stopped \
  --name zot ghcr.io/project-zot/zot:latest

curl http://localhost:5000/v2/_catalog    
```



## 8. VM1 — construir y publicar API1 y API2

```bash
cd api1
sudo nerdctl build -t api1-202112674:v1 .
sudo nerdctl image convert --oci api1-202112674:v1 api1-202112674:v1-oci
sudo nerdctl images | grep api1-202112674     # anota el IMAGE ID de v1-oci
sudo nerdctl tag <IMAGE_ID_OCI> 192.168.122.115:5000/api1-202112674:v1
sudo nerdctl --insecure-registry push 192.168.122.115:5000/api1-202112674:v1

cd ../api2
sudo nerdctl build -t api2-202112674:v1 .
sudo nerdctl image convert --oci api2-202112674:v1 api2-202112674:v1-oci
sudo nerdctl images | grep api2-202112674     # anota el IMAGE ID de v1-oci
sudo nerdctl tag <IMAGE_ID_OCI> 192.168.122.115:5000/api2-202112674:v1
sudo nerdctl --insecure-registry push 192.168.122.115:5000/api2-202112674:v1
```


## 9. VM2 — construir y publicar API3

Configura el registro como inseguro:
```bash
sudo tee -a /etc/containers/registries.conf << 'EOF'
[[registry]]
location = "192.168.122.115:5000"
insecure = true
EOF
```

Construye y publica:
```bash
cd api3
sudo podman build -t api3-202112674:v1 .
sudo podman tag api3-202112674:v1 192.168.122.115:5000/api3-202112674:v1
sudo podman push 192.168.122.115:5000/api3-202112674:v1
```

## 10. VM3 — permitir registro inseguro para Docker (opcional, solo si vas a hacer pull ahí también)

```bash
echo '{ "insecure-registries": ["192.168.122.115:5000"] }' | sudo tee /etc/docker/daemon.json
sudo systemctl restart docker
```

## 12. Verificar que las 3 imágenes están en el registro

```bash
curl http://192.168.122.115:5000/v2/_catalog
```
Debe mostrar `api1-202112674`, `api2-202112674` y `api3-202112674`.

## 13. VM1 — descargar y correr API1 y API2

```bash
sudo nerdctl --insecure-registry pull 192.168.122.115:5000/api1-202112674:v1
sudo nerdctl run -d -p 8081:8081 --name api1 192.168.122.115:5000/api1-202112674:v1

sudo nerdctl --insecure-registry pull 192.168.122.115:5000/api2-202112674:v1
sudo nerdctl run -d -p 8082:8082 --name api2 192.168.122.115:5000/api2-202112674:v1

sudo nerdctl ps
```

## 14. VM2 — descargar y correr API3

```bash
sudo podman pull 192.168.122.115:5000/api3-202112674:v1
sudo podman run -d -p 8083:8083 --name api3 192.168.122.115:5000/api3-202112674:v1

sudo podman ps
```

## 15. Verificación final

```bash
curl http://192.168.122.4:8081/health
curl http://192.168.122.4:8082/health
curl http://192.168.122.44:8083/health

curl http://192.168.122.4:8081/api1/202112674/call-api2
curl http://192.168.122.4:8081/api1/202112674/call-api3
curl http://192.168.122.4:8082/api2/202112674/call-api1
curl http://192.168.122.4:8082/api2/202112674/call-api3
curl http://192.168.122.44:8083/api3/202112674/call-api1
curl http://192.168.122.44:8083/api3/202112674/call-api2
```

Todas las respuestas de `call-apiX` deben incluir `"connection": true`.

