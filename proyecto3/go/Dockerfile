# Uso: docker build --build-arg SERVICE=rest-api -t <imagen> go/
FROM golang:1.26-alpine AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE
RUN CGO_ENABLED=0 go build -o /out/main ./cmd/${SERVICE}

FROM alpine:3.20
# /app/secretos es el directorio de prueba para la regla de Falco (fase posterior)
RUN mkdir -p /app/secretos
COPY --from=build /out/main /app/main
ENTRYPOINT ["/app/main"]
