/* Copyright (c) 2021 Intel Corporation. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or
 * without modification, are permitted provided that the
 * following conditions are met:
 *
 * 1. Redistributions of source code must retain the above
 *    copyright notice, this list of conditions and the
 *    following disclaimer.
 * 2. Redistributions in binary form must reproduce the
 *    above copyright notice, this list of conditions and
 *    the following disclaimer in the documentation and/or
 *    other materials provided with the distribution.
 * 3. Neither the name of the copyright holder nor the names
 *    of its contributors may be used to endorse or promote
 *    products derived from this software without specific
 *    prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS
 * FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE
 * COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT,
 * INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
 * BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
 * LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED
 * AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
 * OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
 * THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH
 * DAMAGE.
 */

#include <unistd.h>
#include <stdlib.h>
#include <stdio.h>
#include <signal.h>
#include <errno.h>
#include <bpf/bpf.h>

#include "sgx.skel.h"

static volatile bool exiting = false;

struct sgx_page_event {
	unsigned long long cgroupid;
	unsigned long long pid;
	int action;
	unsigned long long len;
	void *encl;
};

enum action {
	CREATE,
	DELETE,
};

void sig_handler(int sig)
{
	exiting = true;
}

int handle_event(void *ctx, void *data, size_t data_sz)
{
	const struct sgx_page_event *e = data;
	struct sgx_bpf *skel = ctx;
	char *msg;
	int err;

	int ncpus = libbpf_num_possible_cpus();
	if (ncpus < 0) {
		/* TODO(mythi): error handling */
		return 0;
	}
	unsigned long long values[ncpus];
	unsigned long long sum = 0;

	bpf_map_lookup_elem(bpf_map__fd(skel->maps.container_sgx_epc_usage), &e->cgroupid, &sum);

	switch (e->action) {
		case CREATE:
			sum += e->len;
			msg = "created";
			break;
		case DELETE:
			msg = "deleted";
			bpf_map_lookup_elem(bpf_map__fd(skel->maps.task_sgx_epc_usage), &e->pid, &values);

			for (int i=0; i < ncpus; i++) {
				sum -= values[i];
			}

			err = bpf_map_delete_elem(bpf_map__fd(skel->maps.task_sgx_epc_usage), &e->pid);
			if (err == -1 && errno == ENOENT) {
				return 0;
			}
			break;
	}

	bpf_map_update_elem(bpf_map__fd(skel->maps.container_sgx_epc_usage), &e->cgroupid, &sum, BPF_ANY);

	printf("Container (ID=%llu) %s enclave (SGX EPC usage: %llu)\n", e->cgroupid, msg, sum);

	return 0;
}

#define SGX_EPC_LIMIT_PIN_PATH "/sys/fs/bpf/container_sgx_epc_limit"
#define SGX_EPC_USAGE_PIN_PATH "/sys/fs/bpf/container_sgx_epc_usage"
#define CONTAINER_ID_HASH_PIN_PATH "/sys/fs/bpf/container_id_hash"

int main(void)
{
	struct ring_buffer *seb = NULL;
	struct sgx_bpf *skel = NULL;
	int err = 0;

	/* Clean handling of Ctrl-C */
	signal(SIGINT, sig_handler);
	signal(SIGTERM, sig_handler);

	skel = sgx_bpf__open_and_load();
	if (!skel)
		goto close_prog;

	err = sgx_bpf__attach(skel);
	if (err)
		goto close_prog;

	err = bpf_map__set_pin_path(skel->maps.container_sgx_epc_limit, SGX_EPC_LIMIT_PIN_PATH);
	if (err)
		goto close_prog;

	err = bpf_map__set_pin_path(skel->maps.container_sgx_epc_usage, SGX_EPC_USAGE_PIN_PATH);
	if (err)
		goto close_prog;

	err = bpf_map__set_pin_path(skel->maps.container_id_hash, CONTAINER_ID_HASH_PIN_PATH);
	if (err)
		goto close_prog;

	err = bpf_object__pin_maps(skel->obj, NULL);
	if (err)
		goto close_prog;

	seb = ring_buffer__new(bpf_map__fd(skel->maps.sgx_ringbuf), handle_event, (void *)skel, NULL);
	if (!seb) {
		err = -1;
		fprintf(stderr, "Failed to create SGX events ring buffer\n");
		goto close_prog;
	}

	printf("SGX eBPF snoop prog attached...Hit Ctrl-C to exit.\n");

	while (!exiting) {
		err = ring_buffer__poll(seb, 100 /* timeout, ms */);

		/* Ctrl-C will cause -EINTR */
		if (err == -EINTR) {
			err = 0;
			break;
		}
		if (err < 0) {
			printf("Error polling ring buffer: %d\n", err);
			break;
		}
	}

close_prog:
	printf("SGX eBPF snoop prog exiting (err=%d, errno=%d)\n", err, errno);

	sgx_bpf__destroy(skel);
	ring_buffer__free(seb);

	return err;
}
