package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	containerPIDsMu sync.RWMutex
	containerPIDs   = map[int]bool{}
)

const (
	carnet       = "202112674"
	procPath     = "/proc/continfo_pr2_so1_" + carnet
	cronScript   = "/home/gabriel/proyecto2/cronjob/deploy_containers.sh"
	loadModuleSh = "/home/gabriel/proyecto2/kernel_module/load_module.sh"
	minLowUsage  = 3
	minHighUsage = 2
	loopInterval = 30 * time.Second
	cronUser     = "gabriel"
)

var rdb *redis.Client
var ctx = context.Background()

type ProcInfo struct {
	PID         int
	Name        string
	Cmd         string
	ContainerNS int
	VszKB       int64
	RssKB       int64
	MemPct      float64
	CpuPct      float64
}

func main() {
	fmt.Println("=== Daemon de telemetría de contenedores - Proyecto 2 SO1 ===")

	rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()

	// 1. Inicio del servicio
	startGrafanaAndValkey()
	installCronjob()
	loadKernelModule()

	// 3.5 arrancar eBPF y escuchar eventos de kill en paralelo
	events, cleanupEBPF, err := startEBPF()
	if err != nil {
		fmt.Println("ADVERTENCIA: no se pudo iniciar eBPF:", err)
	} else {
		defer cleanupEBPF()
		go listenKillEvents(events)
	}

	// Manejo de Ctrl+C / SIGTERM para limpiar el cronjob antes de salir
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(loopInterval)
	defer ticker.Stop()

	fmt.Println("Daemon corriendo. Ctrl+C para detener limpiamente.")

	for {
		select {
		case <-ticker.C:
			runManagementCycle()
		case <-sigChan:
			fmt.Println("\nSeñal de terminación recibida, limpiando...")
			removeCronjob()
			fmt.Println("Cronjob eliminado. Saliendo.")
			return
		}
	}
}

// ---------- 1. Inicio del servicio ----------

func startGrafanaAndValkey() {
	fmt.Println("[init] Levantando Grafana + Valkey con docker compose...")
	cmd := exec.Command("docker", "compose", "-f", "../docker-compose.yml", "up", "-d")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println("ERROR levantando docker compose:", err, string(out))
	}
}

func installCronjob() {
	fmt.Println("[init] Instalando cronjob...")
	line := fmt.Sprintf("*/2 * * * * %s", cronScript)
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`(crontab -u %s -l 2>/dev/null | grep -v '%s'; echo "%s") | crontab -u %s -`,
			cronUser, cronScript, line, cronUser))
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println("ERROR instalando cronjob:", err, string(out))
	}
}

func removeCronjob() {
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`crontab -u %s -l 2>/dev/null | grep -v '%s' | crontab -u %s -`,
			cronUser, cronScript, cronUser))
	cmd.Run()
}

func loadKernelModule() {
	fmt.Println("[init] Cargando módulo de kernel...")
	cmd := exec.Command("bash", loadModuleSh)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println("ERROR cargando módulo de kernel:", err, string(out))
	}
}

// ---------- 4. Loop principal ----------

func runManagementCycle() {
	dockerPIDs := dockerRootPIDs()
	sysInfo, procs, err := readProcFile(dockerPIDs)
	if err != nil {
		fmt.Println("ERROR leyendo /proc:", err)
		return
	}

	updateContainerPIDs(procs)

	fmt.Printf("[loop] RAM total=%dKB libre=%dKB uso=%dKB | procesos en contenedores=%d\n",
		sysInfo["RAM_TOTAL_KB"], sysInfo["RAM_LIBRE_KB"], sysInfo["RAM_USO_KB"], len(procs))

	saveSnapshotToValkey(sysInfo, procs)
	saveTop5ToValkey(procs)
	manageContainers(procs)
}

// dockerRootPIDs devuelve el conjunto de PIDs raíz (host) de todos los
// contenedores actualmente corriendo, usando docker inspect.
func dockerRootPIDs() map[int]string {
	out, err := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
	if err != nil {
		return nil
	}
	result := map[int]string{}
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name == "" {
			continue
		}
		pidOut, err := exec.Command("docker", "inspect", "--format", "{{.State.Pid}}", name).Output()
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(pidOut)))
		if err != nil || pid == 0 {
			continue
		}
		result[pid] = name
	}
	return result
}
func readProcFile(dockerPIDs map[int]string) (map[string]int64, []ProcInfo, error) {

	f, err := os.Open(procPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	sysInfo := map[string]int64{}
	var procs []ProcInfo

	reLine := regexp.MustCompile(`PID:(\d+) NAME:(\S+) CMD:(.*?) CONTAINER_NS:(\d+) VSZ_KB:(\d+) RSS_KB:(\d+) MEM_PCT:([\d.]+) CPU_PCT:([\d.]+)`)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "RAM_TOTAL_KB:") {
			sysInfo["RAM_TOTAL_KB"], _ = strconv.ParseInt(strings.TrimSpace(strings.Split(line, ":")[1]), 10, 64)
			continue
		}
		if strings.HasPrefix(line, "RAM_LIBRE_KB:") {
			sysInfo["RAM_LIBRE_KB"], _ = strconv.ParseInt(strings.TrimSpace(strings.Split(line, ":")[1]), 10, 64)
			continue
		}
		if strings.HasPrefix(line, "RAM_USO_KB:") {
			sysInfo["RAM_USO_KB"], _ = strconv.ParseInt(strings.TrimSpace(strings.Split(line, ":")[1]), 10, 64)
			continue
		}

		m := reLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pid, _ := strconv.Atoi(m[1])

		// Filtro real: el PID debe ser el proceso raíz de un contenedor Docker conocido
		containerName, isDocker := dockerPIDs[pid]
		if !isDocker {
			continue
		}

		nsID, _ := strconv.ParseUint(m[4], 10, 64)
		vsz, _ := strconv.ParseInt(m[5], 10, 64)
		rss, _ := strconv.ParseInt(m[6], 10, 64)
		memPct, _ := strconv.ParseFloat(m[7], 64)
		cpuPct, _ := strconv.ParseFloat(m[8], 64)

		procs = append(procs, ProcInfo{
			PID: pid, Name: containerName, Cmd: m[3], ContainerNS: int(nsID),
			VszKB: vsz, RssKB: rss, MemPct: memPct, CpuPct: cpuPct,
		})
	}
	return sysInfo, procs, scanner.Err()
}
func updateContainerPIDs(procs []ProcInfo) {
	containerPIDsMu.Lock()
	defer containerPIDsMu.Unlock()
	containerPIDs = map[int]bool{}
	for _, p := range procs {
		containerPIDs[p.PID] = true
	}
}

func isKnownContainerPID(pid int) bool {
	containerPIDsMu.RLock()
	defer containerPIDsMu.RUnlock()
	return containerPIDs[pid]
}

// ---------- Gestión de contenedores (restricciones clave) ----------

func manageContainers(procs []ProcInfo) {
	names := dockerContainerNames() // map[pid]name, via `docker inspect`
	if len(names) == 0 {
		return
	}

	// Clasifica contenedores activos según el nombre de imagen (alto/bajo consumo)
	var low, high []string
	activeNames := map[string]bool{}
	for _, name := range names {
		activeNames[name] = true
	}

	for name := range activeNames {
		if name == "grafana" || name == "valkey" {
			continue // nunca tocar Grafana
		}
		if isHighConsumption(name) {
			high = append(high, name)
		} else {
			low = append(low, name)
		}
	}

	// Ordena por consumo (RAM+CPU aproximado usando los datos ya leídos del /proc)
	sort.Slice(procs, func(i, j int) bool {
		return (procs[i].RssKB + int64(procs[i].CpuPct*1000)) >
			(procs[j].RssKB + int64(procs[j].CpuPct*1000))
	})

	// Mantener mínimos: 3 bajo consumo, 2 alto consumo; eliminar el resto de excedentes
	extraLow := len(low) - minLowUsage
	extraHigh := len(high) - minHighUsage

	if extraLow > 0 {
		killExcess(low, extraLow)
	}
	if extraHigh > 0 {
		killExcess(high, extraHigh)
	}
}

func isHighConsumption(containerName string) bool {
	out, _ := exec.Command("docker", "inspect", "--format", "{{.Config.Image}}", containerName).Output()
	image := strings.TrimSpace(string(out))
	return strings.Contains(image, "go-client") || strings.Contains(image, "intruso")
}

func dockerContainerNames() map[int]string {
	out, err := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
	if err != nil {
		return nil
	}
	names := map[int]string{}
	i := 0
	for _, n := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n == "" {
			continue
		}
		names[i] = n
		i++
	}
	return names
}

func killExcess(names []string, count int) {
	for i := 0; i < count && i < len(names); i++ {
		name := names[i]
		fmt.Println("[decision] Deteniendo contenedor excedente:", name)

		pidOut, err := exec.Command("docker", "inspect", "--format", "{{.State.Pid}}", name).Output()
		if err != nil {
			fmt.Println("  no se pudo obtener PID de", name, ":", err)
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(pidOut)))
		if err != nil || pid <= 0 {
			fmt.Println("  PID inválido para", name)
			continue
		}

		// Enviamos la señal nosotros mismos (no vía "docker stop"), para que
		// quede capturada con certeza por el tracepoint sys_enter_kill de eBPF,
		// con el propio Daemon como proceso emisor.
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			fmt.Println("  error enviando SIGTERM a PID", pid, ":", err)
		}

		time.Sleep(2 * time.Second)

		// Si sigue vivo tras el SIGTERM, forzamos con SIGKILL
		if err := syscall.Kill(pid, 0); err == nil {
			fmt.Println("  no respondió a SIGTERM, forzando SIGKILL a PID", pid)
			syscall.Kill(pid, syscall.SIGKILL)
			time.Sleep(500 * time.Millisecond)
		}

		// Limpieza a nivel de Docker (el contenedor ya debería estar Exited)
		exec.Command("docker", "rm", "-f", name).Run()
	}
}

// ---------- eBPF: confirmación real de terminación ----------

func listenKillEvents(events chan KillEvent) {
	for evt := range events {
		// Solo nos interesan señales de terminación reales sobre PIDs de contenedor conocidos
		if !isKnownContainerPID(int(evt.TargetPID)) {
			continue
		}
		if evt.Signal != 15 && evt.Signal != 9 { // SIGTERM o SIGKILL
			continue
		}

		fmt.Printf("[eBPF] emisor=%d objetivo=%d señal=%d\n", evt.EmitterPID, evt.TargetPID, evt.Signal)
		key := fmt.Sprintf("evento:kill:%d", time.Now().UnixNano())
		rdb.HSet(ctx, key, map[string]interface{}{
			"emitter_pid": evt.EmitterPID,
			"target_pid":  evt.TargetPID,
			"signal":      evt.Signal,
			"timestamp":   time.Now().Format(time.RFC3339),
		})
		rdb.Incr(ctx, "contador:eventos_ebpf")
		rdb.Incr(ctx, "contador:contenedores_eliminados")
	}
}

// ---------- Persistencia en Valkey ----------

func saveSnapshotToValkey(sysInfo map[string]int64, procs []ProcInfo) {
	key := fmt.Sprintf("snapshot:%d", time.Now().Unix())
	rdb.HSet(ctx, key, map[string]interface{}{
		"ram_total_kb": sysInfo["RAM_TOTAL_KB"],
		"ram_libre_kb": sysInfo["RAM_LIBRE_KB"],
		"ram_uso_kb":   sysInfo["RAM_USO_KB"],
		"timestamp":    time.Now().Format(time.RFC3339),
	})
	rdb.Expire(ctx, key, 24*time.Hour)

	rdb.Set(ctx, "ultima_ram_total", sysInfo["RAM_TOTAL_KB"], 0)
	rdb.Set(ctx, "ultima_ram_libre", sysInfo["RAM_LIBRE_KB"], 0)
	rdb.Set(ctx, "ultima_ram_uso", sysInfo["RAM_USO_KB"], 0)

	for _, p := range procs {
		pkey := fmt.Sprintf("proceso:%d:%d", time.Now().Unix(), p.PID)
		rdb.HSet(ctx, pkey, map[string]interface{}{
			"pid": p.PID, "name": p.Name, "container_ns": p.ContainerNS,
			"rss_kb": p.RssKB, "vsz_kb": p.VszKB,
			"mem_pct": p.MemPct, "cpu_pct": p.CpuPct,
			"timestamp": time.Now().Format(time.RFC3339),
		})
		rdb.Expire(ctx, pkey, 24*time.Hour)
	}
}

func saveTop5ToValkey(procs []ProcInfo) {
	sortedByRAM := make([]ProcInfo, len(procs))
	copy(sortedByRAM, procs)
	sort.Slice(sortedByRAM, func(i, j int) bool { return sortedByRAM[i].RssKB > sortedByRAM[j].RssKB })

	sortedByCPU := make([]ProcInfo, len(procs))
	copy(sortedByCPU, procs)
	sort.Slice(sortedByCPU, func(i, j int) bool { return sortedByCPU[i].CpuPct > sortedByCPU[j].CpuPct })

	top5RAM := map[string]interface{}{}
	for i := 0; i < 5 && i < len(sortedByRAM); i++ {
		label := fmt.Sprintf("PID_%d_NS_%d", sortedByRAM[i].PID, sortedByRAM[i].ContainerNS)
		top5RAM[label] = sortedByRAM[i].RssKB
	}
	if len(top5RAM) > 0 {
		rdb.Del(ctx, "top5_ram")
		rdb.HSet(ctx, "top5_ram", top5RAM)
	}

	top5CPU := map[string]interface{}{}
	for i := 0; i < 5 && i < len(sortedByCPU); i++ {
		label := fmt.Sprintf("PID_%d_NS_%d", sortedByCPU[i].PID, sortedByCPU[i].ContainerNS)
		top5CPU[label] = sortedByCPU[i].CpuPct
	}
	if len(top5CPU) > 0 {
		rdb.Del(ctx, "top5_cpu")
		rdb.HSet(ctx, "top5_cpu", top5CPU)
	}
}
