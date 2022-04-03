#!/bin/sh

dlb_dev=$(ls /dev/dlb* | sed 's/\/dev\/dlb//' | head -1)
echo '\n1. Directed Traffic test'
echo '--------------------------'
/usr/local/bin/dir_traffic -n 128 -d $dlb_dev
echo '\n2. Load Balanced Traffic test'
echo '-------------------------------'
/usr/local/bin/ldb_traffic -n 128 -d $dlb_dev

