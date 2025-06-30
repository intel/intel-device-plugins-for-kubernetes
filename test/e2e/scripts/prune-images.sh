#!/bin/bash

docker system prune -f | tail -2 || {
  echo "Docker prune failed, exiting"
  exit 1
}

exit 0
