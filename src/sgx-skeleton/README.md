# SGX eBPF LSM Hooks

Simple SGX LSM loader using libbpf "skeleton".

## Build

1. bpftool btf dump file /sys/kernel/btf/vmlinux format c > pkg/epchook/hooks/vmlinux.h
2. cd pkg/epchook; go generate -x; cd -
3. bpftool gen skeleton pkg/epchook/sgx_bpf.o > src/sgx-skeleton/sgx.skel.h
4. clang -o sgx-skeleton `pkg-config --libs --cflags /usr/lib/x86_64-linux-gnu/pkgconfig/libbpf.pc` src/sgx-skeleton/sgx_skeleton.c
