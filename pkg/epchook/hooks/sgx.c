/*
 * Copyright (C) 2021 Intel Corporation
 *
 * This program is free software; you can redistribute it and/or modify it
 * under the terms of the GNU General Public License as published by the
 * Free Software Foundation; version 2.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
 * or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
 * for more details.
 *
 * You should have received a copy of the GNU General Public License along
 * with this program; if not, write to the Free Software Foundation, Inc.,
 * 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301, USA.
 */
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>
#include <linux/ioctl.h>
#include <errno.h>

/* TODO(mythi): drop when Linux 5.12 is out */
#ifndef KERNEL_VERSION
    #define KERNEL_VERSION(a,b,c) (((a) << 16) + ((b) << 8) + (c))
#endif

#define SGX_MAGIC 0xA4

#define SGX_IOC_ENCLAVE_CREATE \
	_IOW(SGX_MAGIC, 0x00, struct sgx_enclave_create)
#define SGX_IOC_ENCLAVE_ADD_PAGES \
	_IOWR(SGX_MAGIC, 0x01, struct sgx_enclave_add_pages)
#define SGX_IOC_ENCLAVE_INIT \
	_IOW(SGX_MAGIC, 0x02, struct sgx_enclave_init)

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 256 * 1024 /* 256 KB */);
} sgx_ringbuf SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__uint(max_entries, 1024);
	__type(key, u64);
	__type(value, u64);
} task_sgx_epc_usage SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 1024);
	__type(key, u64);
	__type(value, u64);
} container_sgx_epc_limit SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 1024);
	__type(key, u64);
	__type(value, u64);
} container_sgx_epc_usage SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 1024);
	__type(key, u64);
	__type(value, bool);
} container_sgx_blocked SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 1024);
	__type(key, char[64]);
	__type(value, u64);
} container_id_hash SEC(".maps");

struct sgx_page_event {
	unsigned long long cgroupid;
	unsigned long long pid;
	int action;
	u64 len;
	void *encl;
};

enum action {
	CREATE,
	DELETE,
};

#if 0
SEC("fexit.s/__x64_sys_ioctl")
int BPF_PROG(sgx_ioc_enclave_fexit, struct pt_regs *regs, int ret)
{
	unsigned long arg = PT_REGS_PARM3(regs);
	unsigned int cmd = PT_REGS_PARM2(regs);
	struct sgx_enclave_add_pages add_arg;
	struct sgx_page_event *e;

	if (ret != 0)
		return 0;

	u64 pid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;

	switch (cmd) {
		case SGX_IOC_ENCLAVE_ADD_PAGES:
			e = bpf_ringbuf_reserve(&sgx_ringbuf, sizeof(*e), 0);
			if (!e)
				return 0;

			bpf_copy_from_user(&add_arg, sizeof(add_arg), (void *)arg);

			u64 v = add_arg.length;
			u64 *k = bpf_map_lookup_elem(&task_sgx_epc_usage, &pid);
			if (k) {
				__sync_fetch_and_add(k, v);
			} else {
				bpf_map_update_elem(&task_sgx_epc_usage, &pid, &v, BPF_NOEXIST);
			}

			e->cgroupid = bpf_get_current_cgroup_id();
			e->pid = pid;
			e->action = ADD;
			e->len = v;

			bpf_ringbuf_submit(e, 0);
			break;
	}
	return 0;
}
#endif

SEC("fexit/__x64_sys_ioctl")
int BPF_PROG(sgx_enclave_snoop, struct pt_regs *regs, int ret) {
	unsigned int efd = PT_REGS_PARM1(regs);
	unsigned int cmd = PT_REGS_PARM2(regs);
	struct sgx_page_event *e;
	struct sgx_encl *encl;
	unsigned int page_cnt;
	struct file **fdtable;

	if (ret != 0)
		return 0;

	struct task_struct *task = (struct task_struct *)bpf_get_current_task();
	u64 pid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;

	switch (cmd) {
		case SGX_IOC_ENCLAVE_INIT:
			e = bpf_ringbuf_reserve(&sgx_ringbuf, sizeof(*e), 0);
			if (!e)
				return 0;

			BPF_CORE_READ_INTO(&fdtable, task, files, fdt, fd);

			struct file *f;
			bpf_core_read(&f, sizeof(f), &fdtable[efd]);

			BPF_CORE_READ_INTO(&encl, f, private_data);
			BPF_CORE_READ_INTO(&page_cnt, encl, page_cnt);

			u64 v = 4096 * page_cnt;

			u64 *k = bpf_map_lookup_elem(&task_sgx_epc_usage, &pid);
			if (k) {
				__sync_fetch_and_add(k, v);
			} else {
				bpf_map_update_elem(&task_sgx_epc_usage, &pid, &v, BPF_NOEXIST);
			}

			e->cgroupid = bpf_get_current_cgroup_id();
			e->pid = pid;
			e->action = CREATE;
			e->len = v;
			e->encl = (void *)encl; /* debug */

			bpf_ringbuf_submit(e, 0);
			break;
	}
	return 0;
}

SEC("tracepoint/sched/sched_process_exit")
int BPF_PROG(sched_exit_snoop, void *args) {
	u64 pid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;
	u64 *k = bpf_map_lookup_elem(&task_sgx_epc_usage, &pid);
	struct sgx_page_event *e;

	if (k) {
		e = bpf_ringbuf_reserve(&sgx_ringbuf, sizeof(*e), 0);
		if (!e)
			return 0;

		e->cgroupid = bpf_get_current_cgroup_id();
		e->pid = pid;
		e->action = DELETE;
		e->len = 0;

		bpf_ringbuf_submit(e, 0);
	}

	return 0;
}

SEC("tracepoint/signal/signal_deliver")
int BPF_PROG(signal_deliver_snoop, int sig, struct siginfo *info, struct k_sigaction *ka) {
	int error_code;
	bpf_core_read(&error_code, sizeof(error_code), &info->si_code);
	if (error_code & ~X86_PF_SGX)
		return 0;

	/* SIGSEGV w/ X86_PF_SGX code delivered: enclave re-creation needed */

	u64 pid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;
	u64 *k = bpf_map_lookup_elem(&task_sgx_epc_usage, &pid);
	struct sgx_page_event *e;

	if (k) {
		e = bpf_ringbuf_reserve(&sgx_ringbuf, sizeof(*e), 0);
		if (!e)
			return 0;

		e->cgroupid = bpf_get_current_cgroup_id();
		e->pid = pid;
		e->action = DELETE;
		e->len = 0;

		bpf_ringbuf_submit(e, 0);
	}
	return 0;

}
#if 0
SEC("lsm/file_ioctl")
int BPF_PROG(sgx_epc_hook, struct file *file, unsigned int cmd,
        unsigned long arg, int ret)
{
       if (ret != 0)
               return ret;

       if (cmd == SGX_IOC_ENCLAVE_CREATE) {
	       u64 cgroup_id = bpf_get_current_cgroup_id();
	       bool *block;

	       block = bpf_map_lookup_elem(&container_sgx_blocked, &cgroup_id);
	       if (block)
		       return -EPERM;
       }

       return 0;
}
#endif

char _license[] SEC("license") = "GPL";

unsigned int _version SEC("version") = KERNEL_VERSION(5, 11, 0);
