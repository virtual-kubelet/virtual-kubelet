#!/bin/bash
# Copyright 2016 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the bootstrap filesystem ontop of the base

# exit on failure
set -e

if [ -n "$DEBUG" ]; then
    set -x
fi

DIR=$(dirname $(readlink -f "$0"))
. $DIR/base/utils.sh

function usage() {
echo "Usage: $0 -c yum-cache(tgz) -p base-package(tgz) -o output-package(tgz) -d <activates debug when set>" 1>&2
exit 1
}

while getopts "c:p:o:d:" flag
do
    case $flag in

        p)
            # Required. Package name
            package="$OPTARG"
            ;;

        o)
            # Required. Target for iso and source for components
            OUT="$OPTARG"
            ;;

        d)
            # Optional. directs script to make a debug iso instead of a production iso.
            debug='$OPTARG'
            ;;

        c)
            # Optional. Offline cache of yum packages
            cache="$OPTARG"
            ;;

        *)
            usage
            ;;
    esac
done

shift $((OPTIND-1))

# check there were no extra args and the required ones are set
if [ ! -z "$*" -o -z "$package" -o -z "${OUT}" ]; then
    usage
fi

#################################################################
# Above: arg parsing and setup
# Below: the image authoring
#################################################################

PKGDIR=$(mktemp -d)

unpack $package $PKGDIR

if [ -v debug ]; then
    # These are the packages we install to create an interactive bootstrapVM
    # Install bootstrap base packages
    #
    # packages list here
    #   tndf      # allows package install during debugging.
    #   vim       # basic editing function for debugging.
    yum_cached -c $cache -u -p $PKGDIR install \
        bash \
        shadow \
        tdnf \
        vim \
        -y --nogpgcheck

    # HACK until the issues with override.conf above are dealt with
    pwhash=$(openssl passwd -1 -salt vic password)
    sed -i -e "s/^root:[^:]*:/root:${pwhash}:/" $(rootfs_dir $PKGDIR)/etc/shadow
fi

# Install bootstrap base packages
#
# List stable packages here
#   iproute2  # for ip
#   libtirpc  # due to a previous package reliance on rpc
#
yum_cached -c $cache -u -p $PKGDIR install \
    haveged \
    systemd \
    iptables \
    runc \
    -y --nogpgcheck

# https://www.freedesktop.org/wiki/Software/systemd/InitrdInterface/
touch $(rootfs_dir $PKGDIR)/etc/initrd-release

# ensure we're not including a cache in the staging bundle
# but don't update the cache bundle we're using to install
yum_cached -p $PKGDIR clean all

# package up the result
pack $PKGDIR $OUT
