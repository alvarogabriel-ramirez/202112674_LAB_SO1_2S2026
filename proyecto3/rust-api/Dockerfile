FROM rust:1-slim AS build
WORKDIR /src
COPY Cargo.toml ./
COPY src ./src
RUN cargo build --release

FROM debian:stable-slim
# /app/secretos es el directorio de prueba para la regla de Falco (fase posterior)
RUN mkdir -p /app/secretos
COPY --from=build /src/target/release/api-rust /app/api-rust
EXPOSE 8000
ENTRYPOINT ["/app/api-rust"]
