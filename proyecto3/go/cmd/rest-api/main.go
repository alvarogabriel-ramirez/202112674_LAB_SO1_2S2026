// Go Deployment 1 - contenedor "rest-api":
// recibe el reporte desde la API Rust (HTTP) y lo reenvia al contenedor
// "grpc-client" del mismo pod (localhost).
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Report struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	target := env("GRPC_CLIENT_URL", "http://localhost:9000/send")
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"go-rest-api"}`))
	})
	// net/http atiende cada peticion en su propia goroutine (concurrencia)
	mux.HandleFunc("POST /report", func(w http.ResponseWriter, r *http.Request) {
		var rep Report
		if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
			http.Error(w, "json invalido", http.StatusBadRequest)
			return
		}
		if len(rep.Country) != 3 || rep.Timestamp == "" {
			http.Error(w, "campos invalidos", http.StatusBadRequest)
			return
		}
		log.Printf("[go-rest-api] recibido country=%s aviones=%d barcos=%d", rep.Country, rep.WarplanesInAir, rep.WarshipsInWater)
		body, _ := json.Marshal(rep)
		resp, err := client.Post(target, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("[go-rest-api] error hacia grpc-client: %v", err)
			http.Error(w, "grpc-client no disponible", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= 300 {
			http.Error(w, "error aguas abajo", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Println("[go-rest-api] escuchando en :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
