//go:build ignore
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

char __license[] SEC("license") = "Dual MIT/GPL";

struct kill_event {
    __u32 emitter_pid;
    __u32 target_pid;
    int   signal;
};

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(int));
    __uint(value_size, sizeof(int));
} events SEC(".maps");

// Se adjunta a la entrada de la syscall kill(pid_t pid, int sig)
SEC("tracepoint/syscalls/sys_enter_kill")
int trace_kill(struct trace_event_raw_sys_enter *ctx) {
    struct kill_event evt = {};
    evt.emitter_pid = bpf_get_current_pid_tgid() >> 32;
    evt.target_pid  = (int)ctx->args[0];
    evt.signal      = (int)ctx->args[1];
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &evt, sizeof(evt));
    return 0;
}