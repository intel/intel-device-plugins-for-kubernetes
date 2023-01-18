#!/bin/sh
dlb_dev=$(ls /dev/dlb* | sed 's/\/dev\/dlb//' | head -1)

echo '\n1. Order queue test'
echo '-------------------'
dpdk-test-eventdev --no-huge --vdev="dlb2_event,hwdev_id=`echo $dlb_dev`" -- --test=order_queue --nb_flows 64 --nb_pkts 512  --plcores 1 --wlcores 2-7
echo '\n2. Perf queue test'
echo '------------------'
dpdk-test-eventdev --no-huge --vdev="dlb2_event,hwdev_id=`echo $dlb_dev`" -- --test perf_queue  --nb_flows 64 --nb_pkts 1024 --plcores 1 --wlcores 2-7 --stlist o,a,a,o
echo '\n3. Order atq test'
echo '-----------------'
dpdk-test-eventdev --no-huge --vdev="dlb2_event,hwdev_id=`echo $dlb_dev`" -- --test order_atq   --nb_flows 64 --nb_pkts 1024 --plcores 1 --wlcores 2-7
echo '\n4. Perf atq test'
echo '----------------'
dpdk-test-eventdev --no-huge --vdev="dlb2_event,hwdev_id=`echo $dlb_dev`" -- --test perf_atq    --nb_flows 64 --nb_pkts 1024 --plcores 1 --wlcores 2-7 --stlist o,a,a,o
