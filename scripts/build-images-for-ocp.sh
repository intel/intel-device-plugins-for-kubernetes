#!/usr/bin/env bash
# Copyright 2026 Intel Corporation. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# build-images-for-ocp.sh – builds all container images required for the OCP
# e2e tests from the current project source tree.
#
# Usage:
#   ./scripts/build-images-for-ocp.sh
#
# Environment variables (all optional, match Makefile defaults):
#   BUILDER   – container builder binary: docker (default), podman, or buildah
#   TAG       – image tag (default: value from Makefile, currently 0.35.1)
#   UBI       – set to 1 to build UBI-based images instead of distroless
#
# After a successful run the images are available locally as:
#   intel/<name>:<TAG>
# Pass these to mirror-images-to-ocp.sh to push them to an OCP cluster.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# To load OCP_IMAGES
. "${SCRIPT_DIR}/common-ocp.sh"

log() { echo "[build-images-for-ocp] $*"; }

cd "${REPO_ROOT}"

# Ensure Dockerfiles are up to date with the .Dockerfile.in templates before
# building.  This is a no-op if nothing has changed.
log "Regenerating Dockerfiles from templates..."
make dockerfiles

log "Building images: ${OCP_IMAGES[*]}"
for img in "${OCP_IMAGES[@]}"; do
    log "  building ${img}..."
    make "${img}" ${BUILDER:+BUILDER="${BUILDER}"} ${TAG:+TAG="${TAG}"} ${UBI:+UBI="${UBI}"}
done

# Print a summary so the caller can verify and export OCP_IMAGE_REGISTRY later.
TAG="${TAG:-$(grep '^TAG' Makefile | head -1 | awk -F'[?=]' '{print $NF}' | tr -d ' ')}"
log ""
log "Build complete.  Images available locally:"
for img in "${OCP_IMAGES[@]}"; do
    log "  intel/${img}:${TAG}"
done
log ""
log "Next step: push to your OCP cluster:"
log "  ./scripts/mirror-images-to-ocp.sh <namespace>"
