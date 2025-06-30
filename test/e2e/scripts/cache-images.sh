#!/bin/bash

script_path=$(dirname $(readlink -f $0))

source $script_path/common.sh

cd $GITHUB_WORKSPACE

for img in $IMAGES; do
  echo "Store image to cache: $img:$TAG"
  docker save intel/$img:$TAG | ctr -n k8s.io image import - || exit 1
done
