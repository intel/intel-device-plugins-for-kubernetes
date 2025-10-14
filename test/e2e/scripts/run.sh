#!/bin/bash

preplog="$GITHUB_WORKSPACE/prepare-env.log"
buildlog="$GITHUB_WORKSPACE/build-images.log"

script_path=$(dirname "$(readlink -f "$0")")

source "$script_path/common.sh"

generate_tag

echo "TAG: $TAG"

print_large "Prune old images"
bash "$script_path/prune-images.sh" || {
  echo "Pruning images failed, exiting"
  exit 1
}

print_large "Prepare & build"

fails=0

# Prepare env and build the images in parallel
bash "$script_path/prepare-env.sh" > "$preplog" 2>&1 &
prep_pid=$!

echo "env prepare pid: $prep_pid"

bash "$script_path/build-images.sh" > "$buildlog" 2>&1 &
build_pid=$!

echo "build pid: $build_pid"

wait $prep_pid || {
  echo "Preparation failed, exiting"

  cat "$preplog"

  fails=1
} && {
  print_large "Env ready"
}

wait $build_pid || {
  echo "Build failed, exiting"

  cat "$buildlog"

  fails=1
} && {
  print_large "Build ready"
}

[ $fails -gt 0 ] && {
  exit 1
}

print_large "Cache images"

bash "$script_path/cache-images.sh" || {
  echo "Caching images failed, exiting"
  exit 1
}

print_large "Run tests"

bash "$script_path/run-tests.sh" || {
  echo "FAIL"
  exit 1
}

print_large "PASS"
