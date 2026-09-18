package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	apiName = "API3"
	vmName  = "VM2"
	carnet  = "202112674"
	port    = ":8083"
)

var otherAPIs = map[string]string{
	"api1": "http://192.168.122.4:8081/health",  // API1 - VM1
	"api2": "http://192.168.122.4:8082/health",  // API2 - VM1
}

type HealthResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	VM        string `json:"VM"`
	Carnet    string `json:"carnet"`
}

type CallResponse struct {
	APIName    string `json:"apiname"`
	Message    string `json:"message"`
	Connection bool   `json:"connection"`
	Carnet     string `json:"carnet"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "UP",
		Message:   fmt.Sprintf("%s is Ready", apiName),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		VM:        vmName,
		Carnet:    carnet,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func callAPIHandler(targetKey, targetAPIName, targetVM string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := otherAPIs[targetKey]
		client := http.Client{Timeout: 3 * time.Second}
		res, err := client.Get(url)

		var resp CallResponse
		resp.APIName = targetAPIName
		resp.Carnet = carnet

		if err != nil {
			resp.Message = fmt.Sprintf("ERROR: The %s located on the %s is not working", targetAPIName, targetVM)
			resp.Connection = false
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)

		var health HealthResponse
		if json.Unmarshal(body, &health) == nil && health.Status == "UP" {
			resp.Message = fmt.Sprintf("The %s located on the %s is working", targetAPIName, targetVM)
			resp.Connection = true
		} else {
			resp.Message = fmt.Sprintf("ERROR: The %s located on the %s is not working", targetAPIName, targetVM)
			resp.Connection = false
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	// call-api1 debe consultar a API1 (no a sí misma)
	mux.HandleFunc(fmt.Sprintf("/api3/%s/call-api1", carnet), callAPIHandler("api1", "API1", "VM1"))
	// call-api2 debe consultar a API2 (no a sí misma) - este era el bug corregido
	mux.HandleFunc(fmt.Sprintf("/api3/%s/call-api2", carnet), callAPIHandler("api2", "API2", "VM1"))

	fmt.Printf("%s escuchando en %s\n", apiName, port)
	http.ListenAndServe(port, mux)
}
