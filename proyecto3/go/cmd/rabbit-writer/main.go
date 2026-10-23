// Go Deployment 2 - contenedor "rabbit-writer":
// recibe mensajes por localhost:9001 y los publica en RabbitMQ.
// Concurrencia: pool de N workers (goroutines), cada uno con su propio canal AMQP,
// alimentados por un canal de Go con buffer.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const queueName = "war_reports"

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var published atomic.Int64

func worker(id int, conn *amqp.Connection, jobs <-chan []byte) {
	ch, err := conn.Channel() // un canal AMQP por goroutine (no son thread-safe)
	if err != nil {
		log.Fatalf("worker %d: no se pudo abrir canal: %v", id, err)
	}
	for body := range jobs {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		})
		cancel()
		if err != nil {
			log.Printf("[rabbit-writer] worker %d error publicando: %v", id, err)
			continue
		}
		if n := published.Add(1); n%500 == 0 {
			log.Printf("[rabbit-writer] mensajes publicados: %d", n)
		}
	}
}

func main() {
	url := fmt.Sprintf("amqp://%s:%s@%s/",
		env("RABBIT_USER", "guest"), env("RABBIT_PASS", "guest"), env("RABBIT_HOST", "rabbitmq:5672"))
	workers, _ := strconv.Atoi(env("WORKERS", "8"))

	var conn *amqp.Connection
	var err error
	for i := 0; i < 60; i++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Printf("[rabbit-writer] esperando a RabbitMQ: %v", err)
		time.Sleep(3 * time.Second)
	}
	if conn == nil {
		log.Fatalf("no se pudo conectar a RabbitMQ: %v", err)
	}
	go func() {
		e := <-conn.NotifyClose(make(chan *amqp.Error, 1))
		log.Fatalf("conexion con RabbitMQ cerrada: %v", e) // Kubernetes reinicia el contenedor
	}()

	setup, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := setup.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		log.Fatalf("no se pudo declarar la cola: %v", err)
	}
	setup.Close()

	jobs := make(chan []byte, 5000)
	for i := 0; i < workers; i++ {
		go worker(i, conn, jobs)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /publish", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil || len(body) == 0 {
			http.Error(w, "cuerpo invalido", http.StatusBadRequest)
			return
		}
		select {
		case jobs <- body:
			w.WriteHeader(http.StatusAccepted)
		default:
			http.Error(w, "cola interna llena", http.StatusServiceUnavailable)
		}
	})
	log.Printf("[rabbit-writer] %d workers, escuchando en 127.0.0.1:9001", workers)
	log.Fatal(http.ListenAndServe("127.0.0.1:9001", mux))
}
