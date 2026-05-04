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

# mirror-images-to-ocp.sh – tags and pushes locally-built device-plugin images
# into an OCP internal ImageStream so that e2e tests run without depending on
# any external registry.
#
# Build the images first with:
#   ./scripts/build-images-for-ocp.sh
#
# Usage:
#   ./scripts/mirror-images-to-ocp.sh <namespace>

set -euo pipefail

log() { echo "[mirror] $*"; }

resolve_registry_route() {
    if [[ -n "${REGISTRY_ROUTE:-}" ]]; then
        printf '%s\n' "${REGISTRY_ROUTE}"
        return 0
    fi

    local route_host=""
    route_host="$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}' 2>/dev/null || true)"

    if [[ -n "${route_host}" ]]; then
        printf '%s\n' "${route_host}"
        return 0
    fi

    echo "ERROR: unable to determine the OpenShift registry route." >&2
    echo "Set REGISTRY_ROUTE explicitly or expose the image registry default route." >&2

    return 1
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
NAMESPACE="${1:?Usage: $0 <namespace>}"

# Get registry route from the cluster to avoid hardcoding it.
# Prefer an explicit env var and otherwise use the image registry default route.
log "Getting registry route from cluster..."

REGISTRY_ROUTE="$(resolve_registry_route)"

DST="${REGISTRY_ROUTE}/${NAMESPACE}"

BUILDER="${BUILDER:-docker}"

PUSHER_SA="registry-pusher"

# Resolve TAG: honour the env var, otherwise fall back to the Makefile default.
if [[ -z "${TAG:-}" ]]; then
    TAG="$(grep '^TAG' "${REPO_ROOT}/Makefile" | head -1 | awk -F'[?=]' '{print $NF}' | tr -d ' ')"
fi

# To load OCP_OCP_IMAGES
. "${SCRIPT_DIR}/common-ocp.sh"

# Verify that all images are present locally before touching the cluster.
log "Verifying locally-built images (tag: ${TAG})..."
missing=()
for name in "${OCP_IMAGES[@]}"; do
    if ! "${BUILDER}" image inspect "intel/${name}:${TAG}" &>/dev/null; then
        missing+=("intel/${name}:${TAG}")
    fi
done

if [[ ${#missing[@]} -gt 0 ]]; then
    echo "ERROR: the following images are not available locally:" >&2
    printf '  %s\n' "${missing[@]}" >&2
    echo "Run ./scripts/build-images-for-ocp.sh first." >&2
    exit 1
fi

# Ensure the namespace/project exists before creating ImageStreams.
if ! oc get namespace "${NAMESPACE}" &>/dev/null; then
    log "Creating namespace ${NAMESPACE}..."
    oc new-project "${NAMESPACE}" || oc create namespace "${NAMESPACE}"
fi

# Create user for ImageStream pushing and get a token for authentication.
log "Setting up permissions for pushing to ${DST}..."
if ! oc get sa "${PUSHER_SA}" -n "${NAMESPACE}" &>/dev/null; then
    oc create sa "${PUSHER_SA}" -n $NAMESPACE &&
    oc adm policy add-role-to-user registry-editor \
    -z "${PUSHER_SA}" -n $NAMESPACE || {
        echo "Failed to set up permissions for pushing to ${DST}, exiting" >&2
        exit 1
}
fi

export TOKEN=$(oc create token "${PUSHER_SA}" -n $NAMESPACE)

# Log in to the OCP internal registry using the current oc session token.
log "Logging in to ${REGISTRY_ROUTE}..."

echo -n "$TOKEN" | "${BUILDER}" login  \
    -u registry-pusher --password-stdin "${REGISTRY_ROUTE}"

# Ensure every ImageStream exists before pushing.
for name in "${OCP_IMAGES[@]}"; do
    if ! oc get imagestream "${name}" -n "${NAMESPACE}" &>/dev/null; then
        log "Creating ImageStream ${name}..."
        oc create imagestream "${name}" -n "${NAMESPACE}"
    fi
done

# Tag and push each locally-built image.
for name in "${OCP_IMAGES[@]}"; do
    src="intel/${name}:${TAG}"
    dst="${DST}/${name}:${TAG}"

    log "Pushing ${src} → ${dst}"
    "${BUILDER}" tag "${src}" "${dst}"
    "${BUILDER}" push "${dst}"
done

log ""
log "All images pushed to ${DST}"
log ""
log "Pods inside the cluster pull images via the internal registry."
log "Export PROJECT_NAMESPACE, IMAGE_PATH & PLUGIN_VERSION before running OCP e2e tests:"
log "  export PROJECT_NAMESPACE=${NAMESPACE}"
log "  export IMAGE_PATH=${NAMESPACE}"
log "  export PLUGIN_VERSION=${TAG}"
