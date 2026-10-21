package main

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
)

type KillEvent struct {
	EmitterPID uint32
	TargetPID  uint32
	Signal     int32
}

// startEBPF carga el programa eBPF, lo adjunta al tracepoint de kill,
// y devuelve un canal por el que llegan los eventos capturados.
func startEBPF() (chan KillEvent, func(), error) {
	var objs KillTraceObjects
	if err := LoadKillTraceObjects(&objs, nil); err != nil {
		return nil, nil, fmt.Errorf("cargando objetos eBPF: %w", err)
	}

	tp, err := link.Tracepoint("syscalls", "sys_enter_kill", objs.TraceKill, nil)
	if err != nil {
		objs.Close()
		return nil, nil, fmt.Errorf("adjuntando tracepoint: %w", err)
	}

	reader, err := perf.NewReader(objs.Events, 4096)
	if err != nil {
		tp.Close()
		objs.Close()
		return nil, nil, fmt.Errorf("creando perf reader: %w", err)
	}

	events := make(chan KillEvent, 100)

	go func() {
		for {
			record, err := reader.Read()
			if err != nil {
				return // el reader se cerró, salimos limpio
			}
			if len(record.RawSample) < 12 {
				continue
			}
			evt := KillEvent{
				EmitterPID: binary.LittleEndian.Uint32(record.RawSample[0:4]),
				TargetPID:  binary.LittleEndian.Uint32(record.RawSample[4:8]),
				Signal:     int32(binary.LittleEndian.Uint32(record.RawSample[8:12])),
			}
			events <- evt
		}
	}()

	cleanup := func() {
		reader.Close()
		tp.Close()
		objs.Close()
	}

	log.Println("eBPF: tracepoint de sys_kill activo")
	return events, cleanup, nil
}
