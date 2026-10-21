FROM alpine:latest
CMD sh -c "while true; do \
    echo '--- Intento de lectura de archivos sensibles ---'; \
    cat /etc/passwd 2>&1 || true; \
    cat /etc/shadow 2>&1 || true; \
    sleep 10; \
done"