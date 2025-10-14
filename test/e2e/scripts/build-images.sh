#!/bin/bash

script_path=$(dirname "$(readlink -f "$0")")
source "$script_path/common.sh"

cd "$GITHUB_WORKSPACE" || exit 1

make set-version

print_large "Build and cache images"

prepare_to_build

for img in $IMAGES; do
  echo "Building $img with tag $TAG"
  make "$img" || exit 1
done

print_large "build ok"

exit 0

