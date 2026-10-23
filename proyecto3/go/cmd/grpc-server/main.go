// Go Deployment 2 - contenedor "grpc-server":
// recibe SendReport por gRPC y se lo entrega al contenedor "rabbit-writer"
// del mismo pod (localhost:9001).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	pb "mumnk8s/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedWarReportServiceServer
	client    *http.Client
	writerURL string
}

func (s *server) SendReport(ctx context.Context, req *pb.WarReportRequest) (*pb.WarReportResponse, error) {
	payload := map[string]any{
		"country":           strings.ToUpper(pb.Countries_name[int32(req.GetCountry())]),
		"warplanes_in_air":  req.GetWarplanesInAir(),
		"warships_in_water": req.GetWarshipsInWater(),
		"timestamp":         req.GetTimestamp(),
	}
	b, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.writerURL, bytes.NewReader(b))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "request invalido: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "writer no disponible: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return nil, status.Errorf(codes.Internal, "writer respondio %d", resp.StatusCode)
	}
	log.Printf("[go-grpc-server] reporte recibido por gRPC country=%v -> writer", payload["country"])
	return &pb.WarReportResponse{Status: "queued"}, nil
}

func main() {
	writerURL := os.Getenv("WRITER_URL")
	if writerURL == "" {
		writerURL = "http://localhost:9001/publish"
	}
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("no se pudo escuchar: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterWarReportServiceServer(s, &server{
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: &http.Transport{MaxIdleConns: 200, MaxIdleConnsPerHost: 200},
		},
		writerURL: writerURL,
	})
	log.Println("[go-grpc-server] gRPC escuchando en :50051")
	log.Fatal(s.Serve(lis))
}
