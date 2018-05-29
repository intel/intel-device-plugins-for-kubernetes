#!/bin/sh -xe

if [ $(cat /sys/class/fpga/intel-fpga-dev.0/intel-fpga-port.0/afu_id) != 'd8424dc4a4a3c413f89e433683f9040b' ] ; then
    echo 'ERROR: NLB(Native Loopback Adapter) AFU is not loaded' >&2
    echo 'Please refer to the NLB documentation for the location of the NLB green bitstream.'
    exit 1
fi

if [ $(cat /sys/kernel/mm/hugepages/hugepages-2048kB/nr_hugepages) -lt 20 ] ; then
    echo 'ERROR: system hugepage is not configured to reserve 2M-hugepages' >&2
    echo 'please run this command to do it: "echo 20 > /sys/kernel/mm/hugepages/hugepages-2048kB/nr_hugepages"'
    exit 1
fi

devnum=$(ls /dev/intel-fpga-port.* | cut -f2 -d.)
name="intel-fpga-dev.$devnum"
bus="0x$(ls -la /sys/class/fpga/$name/ |grep device |cut -f3 -d:)"
echo "using device $name, bus $bus"

/usr/bin/hello_fpga -b $bus
