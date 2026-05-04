# Images required for the OCP e2e tests (operator + QAT + SGX + DSA plugins + workload containers).
OCP_IMAGES=(
    intel-deviceplugin-operator
    intel-qat-plugin
    intel-qat-initcontainer
    intel-sgx-plugin
    intel-dsa-plugin
    intel-iaa-plugin
    intel-gpu-plugin
    intel-idxd-config-initcontainer
    sgx-sdk-demo
    crypto-perf
    dsa-dpdk-dmadevtest
)
