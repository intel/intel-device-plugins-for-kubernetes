#!/bin/sh -e

BUILDER=$1

# check if DCP tarball is present
DCP_TARBALL=a10_gx_pac_ias_1_1_pv_rte_installer.tar.gz
if [ ! -f ${DCP_TARBALL} ] ; then
    echo "ERROR: 'Acceleration Stack for Runtime' tarball $DCP_TARBALL not present"
    echo "ERROR: Please, download it from https://www.intel.com/content/www/us/en/programmable/solutions/acceleration-hub/downloads.html and run this script again"
    exit 1
fi

# build CRI hook
HOOK=fpga_crihook
make -C ../../ $HOOK
cp ../../cmd/$HOOK/$HOOK ./intel-fpga-crihook

# build initcontainer image
WORKSPACE=`realpath .`
IMG="intel-fpga-initcontainer"
TAG=$(git rev-parse HEAD)

if [ -z "${BUILDER}" -o "${BUILDER}" = 'docker' ] ; then
    docker build --rm -t $IMG:devel -f $WORKSPACE/$IMG.Dockerfile $WORKSPACE
    docker tag $IMG:devel $IMG:$TAG
elif [ "${BUILDER}" = 'buildah' ] ; then
    buildah bud -t $IMG:devel -f $WORKSPACE/$IMG.Dockerfile --layers $WORKSPACE
    buildah tag $IMG:devel $IMG:$TAG
else
    (>&2 echo "Unknown builder ${BUILDER}")
    exit 1
fi
