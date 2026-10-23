use std::time::Duration;

use axum::{extract::State, http::StatusCode, routing::{get, post}, Json, Router};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

#[derive(Debug, Deserialize, Serialize)]
struct Report {
    country: String,
    warplanes_in_air: i32,
    warships_in_water: i32,
    timestamp: String,
}

#[derive(Clone)]
struct AppState {
    client: reqwest::Client,
    go_url: String,
}

async fn health() -> Json<Value> {
    Json(json!({"status": "ok", "service": "api-rust"}))
}

async fn receive(State(st): State<AppState>, Json(r): Json<Report>) -> (StatusCode, Json<Value>) {
    if r.country.len() != 3 || r.timestamp.is_empty() {
        return (StatusCode::BAD_REQUEST, Json(json!({"error": "campos invalidos"})));
    }
    println!(
        "[api-rust] recibido country={} aviones={} barcos={}",
        r.country, r.warplanes_in_air, r.warships_in_water
    );
    match st.client.post(&st.go_url).json(&r).send().await {
        Ok(resp) if resp.status().is_success() => {
            (StatusCode::OK, Json(json!({"status": "ok"})))
        }
        Ok(resp) => {
            eprintln!("[api-rust] go-rest-api respondio {}", resp.status());
            (StatusCode::BAD_GATEWAY, Json(json!({"error": "error en go-rest-api"})))
        }
        Err(e) => {
            eprintln!("[api-rust] no se pudo contactar go-rest-api: {e}");
            (StatusCode::BAD_GATEWAY, Json(json!({"error": "go-rest-api no disponible"})))
        }
    }
}

#[tokio::main]
async fn main() {
    let go_url = std::env::var("GO_API_URL")
        .unwrap_or_else(|_| "http://go-rest-api:8080/report".to_string());
    let route = std::env::var("ROUTE_PATH").unwrap_or_else(|_| "/grpc-202112674".to_string());

    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(5))
        .pool_max_idle_per_host(200)
        .build()
        .expect("cliente http");

    let app = Router::new()
        .route("/health", get(health))
        .route(&route, post(receive))
        .with_state(AppState { client, go_url });

    let listener = tokio::net::TcpListener::bind("0.0.0.0:8000").await.expect("bind");
    println!("[api-rust] escuchando en :8000 ruta {route}");
    axum::serve(listener, app).await.expect("server");
}
