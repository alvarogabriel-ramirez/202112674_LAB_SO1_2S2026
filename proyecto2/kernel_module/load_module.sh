#!/bin/bash
set -e
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

# Si ya está cargado, lo descargamos primero (para poder recompilar sin reiniciar)
if lsmod | grep -q "^continfo"; then
    sudo rmmod continfo
fi

make clean
make
sudo insmod continfo.ko
echo "Modulo continfo cargado correctamente."