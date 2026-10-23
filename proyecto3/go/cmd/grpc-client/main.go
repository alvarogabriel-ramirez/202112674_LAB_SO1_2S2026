// Go Deployment 1 - contenedor "grpc-client":
// recibe JSON por localhost:9000 y llama al servidor gRPC.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	pb "mumnk8s/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	addr := env("GRPC_SERVER_ADDR", "dns:///go-grpc-server:50051")

	// Service headless + round_robin: reparte las llamadas entre las replicas del servidor
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`),
	)
	if err != nil {
		log.Fatalf("no se pudo crear el cliente gRPC: %v", err)
	}
	defer conn.Close()
	client := pb.NewWarReportServiceClient(conn) // la conexion es segura entre goroutines

	mux := http.NewServeMux()
	mux.HandleFunc("POST /send", func(w http.ResponseWriter, r *http.Request) {
		var rep Report
		if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
			http.Error(w, "json invalido", http.StatusBadRequest)
			return
		}
		code, ok := pb.Countries_value[strings.ToLower(rep.Country)]
		if !ok {
			http.Error(w, "pais no soportado", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := client.SendReport(ctx, &pb.WarReportRequest{
			Country:         pb.Countries(code),
			WarplanesInAir:  rep.WarplanesInAir,
			WarshipsInWater: rep.WarshipsInWater,
			Timestamp:       rep.Timestamp,
		})
		if err != nil {
			log.Printf("[go-grpc-client] error gRPC: %v", err)
			http.Error(w, "servidor gRPC no disponible", http.StatusBadGateway)
			return
		}
		log.Printf("[go-grpc-client] enviado country=%s respuesta=%s", rep.Country, res.GetStatus())
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Println("[go-grpc-client] escuchando en 127.0.0.1:9000")
	log.Fatal(http.ListenAndServe("127.0.0.1:9000", mux))
}
