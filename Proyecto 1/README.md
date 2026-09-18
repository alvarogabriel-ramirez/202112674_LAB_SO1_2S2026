#  SO1: Contenedores en Entornos Virtualizados

**Universidad de San Carlos de Guatemala** 

Facultad de Ingeniería 

Ingeniería en Ciencias y Sistemas

Sistemas Operativos 1 

Carnet: **202112674** 

Entorno virtualizado con 3 máquinas virtuales (KVM), cada una corriendo un runtime de contenedores distinto (containerd, Podman, Docker), 3 APIs REST en Go que se comunican entre sí, y un registro privado de imágenes (Zot) para su distribución.

---

## Arquitectura

```
Host Linux (KVM/QEMU)
 ├── VM1 (192.168.122.4)   → containerd → API1 (:8081), API2 (:8082)
 ├── VM2 (192.168.122.44)  → Podman     → API3 (:8083)
 └── VM3 (192.168.122.115) → Docker     → Zot registry (:5000)
```

Cada API expone:
- `GET /health` — estado de la API.
- `GET /apiX/202112674/call-apiY` — verifica, vía REST, si otra API está activa.

Ver el diagrama completo y el detalle técnico en [`docs/MANUAL_TECNICO.md`](docs/MANUAL_TECNICO.md).

---
## Tecnologías utilizadas

- **Virtualización:** KVM/QEMU + libvirt
- **Lenguaje:** Go 1.22
- **Runtimes de contenedores:** containerd (con nerdctl), Podman, Docker
- **Registro de imágenes:** [Zot](https://zotregistry.dev/)
- **SO base de las VMs:** Ubuntu Server 22.04 LTS

---


## Inicio rápido

Para levantar el entorno completo desde cero, sigue [`docs/GUIA_INSTALACION.md`](docs/GUIA_INSTALACION.md).

Resumen del flujo:
1. Crear las 3 VMs con KVM.
2. Instalar containerd (VM1), Podman (VM2), Docker (VM3).
3. Levantar el registro Zot en VM3.
4. Construir las imágenes de las 3 APIs y subirlas a Zot.
5. Descargar (pull) y correr cada API en su VM correspondiente.
6. Probar la comunicación cruzada entre las 3 APIs.

---

## Probar el sistema

```bash
# Salud individual
curl http://192.168.122.4:8081/health
curl http://192.168.122.4:8082/health
curl http://192.168.122.44:8083/health

# Comunicación cruzada
curl http://192.168.122.4:8081/api1/202112674/call-api2
curl http://192.168.122.4:8081/api1/202112674/call-api3
curl http://192.168.122.4:8082/api2/202112674/call-api1
curl http://192.168.122.4:8082/api2/202112674/call-api3
curl http://192.168.122.44:8083/api3/202112674/call-api1
curl http://192.168.122.44:8083/api3/202112674/call-api2
```

Respuesta esperada de un `call-apiX` exitoso:
```json
{"apiname":"API2","message":"The API2 located on the VM1 is working","connection":true,"carnet":"202112674"}
```

---

## Nombres de imágenes

| API | Imagen | VM de build | Runtime |
|---|---|---|---|
| API1 | `api1-202112674:v1` | VM1 | containerd (nerdctl) |
| API2 | `api2-202112674:v1` | VM1 | containerd (nerdctl) |
| API3 | `api3-202112674:v1` | VM2 | Podman |




