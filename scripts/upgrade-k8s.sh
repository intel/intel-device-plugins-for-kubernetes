#!/bin/sh -eu

# based on the work discussed in
# https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-521493597

if [ $# != 1 ] || [ "$1" = "?" ] || [ "$1" = "--help" ]; then
    echo "Usage: $0 <k8s version>" >&2
    exit 1
fi

VERSION="$1"

for MOD in $(
    curl -sS https://raw.githubusercontent.com/kubernetes/kubernetes/v${VERSION}/go.mod |
    sed -n 's|.*k8s.io/\(.*\) => ./staging/src/k8s.io/.*|k8s.io/\1|p'
); do
    echo "$MOD"
    V=$(
        go mod download -json "${MOD}@kubernetes-${VERSION}" |
        sed -n 's|.*"Version": "\(.*\)".*|\1|p'
    )
    go mod edit "-replace=${MOD}=${MOD}@${V}"
done
go get "k8s.io/kubernetes@v${VERSION}"
