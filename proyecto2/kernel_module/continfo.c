// continfo.c
#include <linux/init.h>
#include <linux/module.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/sched.h>
#include <linux/sched/signal.h>
#include <linux/sched/mm.h>
#include <linux/mm.h>
#include <linux/pid_namespace.h>
#include <linux/jiffies.h>
#include <linux/uaccess.h>

#define CARNET "202112674"
#define PROC_NAME "continfo_pr2_so1_" CARNET

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Estudiante SO1");
MODULE_DESCRIPTION("Sonda de kernel para telemetria de contenedores - Proyecto 2 SO1");

static struct proc_dir_entry *proc_entry;

// Extrae la línea de comando (argv) de un proceso, si es posible.
// Si falla, se usa el nombre corto (task->comm) como respaldo.
static void get_task_cmdline(struct task_struct *task, char *buf, size_t buflen)
{
    struct mm_struct *mm;
    unsigned long arg_start, arg_end;
    int len;

    buf[0] = '\0';
    mm = get_task_mm(task);
    if (!mm) {
        snprintf(buf, buflen, "[%s]", task->comm);
        return;
    }

    arg_start = mm->arg_start;
    arg_end = mm->arg_end;
    len = arg_end - arg_start;
    if (len <= 0 || len >= buflen) {
        snprintf(buf, buflen, "%s", task->comm);
        mmput(mm);
        return;
    }

    if (access_process_vm(task, arg_start, buf, len, 0) <= 0) {
        snprintf(buf, buflen, "%s", task->comm);
    } else {
        buf[len] = '\0';
        // Los argumentos vienen separados por \0 -> los convertimos a espacios
        for (int i = 0; i < len - 1; i++) {
            if (buf[i] == '\0') buf[i] = ' ';
        }
    }
    mmput(mm);
}

static int continfo_show(struct seq_file *m, void *v)
{
    struct sysinfo si;
    unsigned long total_kb, free_kb, used_kb;
    struct task_struct *task;
    char cmdline[256];

    si_meminfo(&si);
    total_kb = si.totalram * (PAGE_SIZE / 1024);
    free_kb  = si.freeram  * (PAGE_SIZE / 1024);
    used_kb  = total_kb - free_kb;

    seq_printf(m, "=== MEMORIA DEL SISTEMA ===\n");
    seq_printf(m, "RAM_TOTAL_KB: %lu\n", total_kb);
    seq_printf(m, "RAM_LIBRE_KB: %lu\n", free_kb);
    seq_printf(m, "RAM_USO_KB: %lu\n", used_kb);
    seq_printf(m, "=== PROCESOS ===\n");

    rcu_read_lock();
    for_each_process(task) {
        struct mm_struct *mm;
        unsigned long vsz_kb = 0, rss_kb = 0;
        unsigned long mem_pct_x100 = 0; // porcentaje * 100 para 2 decimales sin floats
        unsigned long cpu_time_ticks;
        unsigned long cpu_pct_x100 = 0;
        int ns_level;
        unsigned int container_ns_id = 0;

        mm = task->mm;
        if (!mm)
            continue; // procesos kernel (sin espacio de memoria de usuario)

        vsz_kb = mm->total_vm * (PAGE_SIZE / 1024);
        rss_kb = get_mm_rss(mm) * (PAGE_SIZE / 1024);

        if (total_kb > 0)
            mem_pct_x100 = (rss_kb * 10000) / total_kb;

        // %CPU aproximado: tiempo de CPU consumido vs tiempo total desde boot
        // (el enunciado permite que sea un numero grande por los calculos diferenciales)
        cpu_time_ticks = task->utime + task->stime; // en unidades de tiempo del kernel
        if (jiffies > 0)
            cpu_pct_x100 = (cpu_time_ticks * 10000) / jiffies;

        // Namespace de PID: si el nivel es > 0, el proceso vive dentro de un
        // contenedor (namespace de PID distinto al del host / init).
        ns_level = task_active_pid_ns(task)->level;
        if (ns_level > 0) {
            container_ns_id = task_active_pid_ns(task)->ns.inum;
        }

        get_task_cmdline(task, cmdline, sizeof(cmdline));

        seq_printf(m, "PID:%d NAME:%s CMD:%s CONTAINER_NS:%u VSZ_KB:%lu RSS_KB:%lu MEM_PCT:%lu.%02lu CPU_PCT:%lu.%02lu\n",
           task->pid, task->comm, cmdline, container_ns_id,
           vsz_kb, rss_kb,
           mem_pct_x100 / 100, mem_pct_x100 % 100,
           cpu_pct_x100 / 100, cpu_pct_x100 % 100);
    }
    rcu_read_unlock();

    return 0;
}

static int continfo_open(struct inode *inode, struct file *file)
{
    return single_open(file, continfo_show, NULL);
}

static const struct proc_ops continfo_fops = {
    .proc_open    = continfo_open,
    .proc_read    = seq_read,
    .proc_lseek   = seq_lseek,
    .proc_release = single_release,
};

static int __init continfo_init(void)
{
    proc_entry = proc_create(PROC_NAME, 0444, NULL, &continfo_fops);
    if (!proc_entry) {
        pr_err("continfo: no se pudo crear /proc/%s\n", PROC_NAME);
        return -ENOMEM;
    }
    pr_info("continfo: modulo cargado, /proc/%s disponible\n", PROC_NAME);
    return 0;
}

static void __exit continfo_exit(void)
{
    proc_remove(proc_entry);
    pr_info("continfo: modulo descargado\n");
}

module_init(continfo_init);
module_exit(continfo_exit);