// Consumer: lee los reportes de RabbitMQ y guarda las estadisticas en Valkey.
// Concurrencia: N workers (goroutines) consumen del mismo canal AMQP con ack manual.
// Cada reporte se procesa con un script Lua (atomico en Valkey), asi varias
// replicas del consumer pueden escribir a la vez sin pisarse.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

const queueName = "war_reports"

type Report struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

// Modelo de datos en Valkey (lo que lee Grafana):
//   totals:planes / totals:ships   HASH pais -> suma acumulada      (Top de paises)
//   reports:count                  HASH pais -> cantidad de reportes
//   stats:max_planes, stats:min_planes, stats:max_ships, stats:min_ships   (maximos y minimos generales)
//   freq:planes / freq:ships       HASH valor -> veces que aparecio
//   stats:mode_planes / stats:mode_ships                            (moda)
//   stream:<PAIS>                  STREAM con planes y ships        (serie temporal; TTL 24h, max 10000 entradas)
const luaScript = `
local c = ARGV[1]
local p = tonumber(ARGV[2])
local s = tonumber(ARGV[3])

redis.call('HINCRBY', 'totals:planes', c, p)
redis.call('HINCRBY', 'totals:ships', c, s)
redis.call('HINCRBY', 'reports:count', c, 1)

local function minmax(prefix, v)
  local mx = redis.call('GET', 'stats:max_' .. prefix)
  if (not mx) or v > tonumber(mx) then redis.call('SET', 'stats:max_' .. prefix, v) end
  local mn = redis.call('GET', 'stats:min_' .. prefix)
  if (not mn) or v < tonumber(mn) then redis.call('SET', 'stats:min_' .. prefix, v) end
end
minmax('planes', p)
minmax('ships', s)

local function mode(prefix, v)
  local f = redis.call('HINCRBY', 'freq:' .. prefix, v, 1)
  local best = redis.call('GET', 'stats:mode_' .. prefix .. '_count')
  if (not best) or f > tonumber(best) then
    redis.call('SET', 'stats:mode_' .. prefix .. '_count', f)
    redis.call('SET', 'stats:mode_' .. prefix, v)
  end
end
mode('planes', p)
mode('ships', s)

local key = 'stream:' .. c
redis.call('XADD', key, 'MAXLEN', '~', 10000, '*', 'planes', p, 'ships', s)
redis.call('EXPIRE', key, 86400)
return 1
`

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var processed atomic.Int64

func worker(id int, msgs <-chan amqp.Delivery, rdb *redis.Client, script *redis.Script) {
	for d := range msgs {
		var r Report
		if err := json.Unmarshal(d.Body, &r); err != nil || len(r.Country) != 3 {
			log.Printf("[consumer] mensaje invalido descartado: %v", err)
			d.Ack(false)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := script.Run(ctx, rdb, nil, r.Country, r.WarplanesInAir, r.WarshipsInWater, r.Timestamp).Err()
		cancel()
		if err != nil {
			log.Printf("[consumer] worker %d error guardando en Valkey: %v (se reencola)", id, err)
			d.Nack(false, true)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		d.Ack(false)
		if n := processed.Add(1); n%500 == 0 {
			log.Printf("[consumer] reportes procesados: %d", n)
		}
	}
}

func main() {
	workers, _ := strconv.Atoi(env("WORKERS", "8"))
	rdb := redis.NewClient(&redis.Options{Addr: env("VALKEY_ADDR", "valkey:6379"), PoolSize: workers * 2})

	for i := 0; i < 60; i++ {
		if err := rdb.Ping(context.Background()).Err(); err == nil {
			break
		} else {
			log.Printf("[consumer] esperando a Valkey: %v", err)
			time.Sleep(3 * time.Second)
		}
	}
	script := redis.NewScript(luaScript)

	url := fmt.Sprintf("amqp://%s:%s@%s/",
		env("RABBIT_USER", "guest"), env("RABBIT_PASS", "guest"), env("RABBIT_HOST", "rabbitmq:5672"))
	var conn *amqp.Connection
	var err error
	for i := 0; i < 60; i++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Printf("[consumer] esperando a RabbitMQ: %v", err)
		time.Sleep(3 * time.Second)
	}
	if conn == nil {
		log.Fatalf("no se pudo conectar a RabbitMQ: %v", err)
	}
	go func() {
		e := <-conn.NotifyClose(make(chan *amqp.Error, 1))
		log.Fatalf("conexion con RabbitMQ cerrada: %v", e)
	}()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		log.Fatal(err)
	}
	if err := ch.Qos(200, 0, false); err != nil {
		log.Fatal(err)
	}
	msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < workers; i++ {
		go worker(i, msgs, rdb, script)
	}
	log.Printf("[consumer] %d workers consumiendo la cola %q", workers, queueName)
	select {} // bloquea para siempre; los workers hacen el trabajo
}
