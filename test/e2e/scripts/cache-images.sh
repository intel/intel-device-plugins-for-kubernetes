#!/bin/bash

cd "$GITHUB_WORKSPACE" || exit 1

for img in $IMAGES; do
  echo "Store image to cache: $img:$TAG"
  docker save intel/$img:$TAG | ctr -n k8s.io image import - || exit 1
done
