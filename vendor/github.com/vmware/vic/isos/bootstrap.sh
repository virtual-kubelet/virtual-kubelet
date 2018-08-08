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

# File list to estimate the size of the target tempfs in bootstrap
tempfs_target_list=('/lib/modules/*' \
    '/bin/tether' \
    '/bin/unpack' \
    '/sbin/*tables*' \
    '/lib/libm.*'\
    '/lib/libm-*' \
    '/lib/libgcc_s*' \
    '/lib/libip*tc*' \
    '/lib/libxtables*' \
    '/lib/libdl*' \
    '/lib/libc.so*'\
    '/lib/libc-*' \
    '/lib64/ld-*' \
    '/usr/lib/iptables' \
    '/lib/libhavege.so.1' \
    '/usr/sbin/haveged')

# Build the bootstrap filesystem ontop of the base

# exit on failure
set -e

if [ -n "$DEBUG" ]; then
    set -x
fi

DIR=$(dirname $(readlink -f "$0"))
. $DIR/base/utils.sh

function usage() {
echo "Usage: $0 -p staged-package(tgz) -b binary-dir -d <activates debug when set>" 1>&2
exit 1
}

while getopts "p:b:d:" flag
do
    case $flag in

        p)
            # Required. Package name
            package="$OPTARG"
            ;;

        b)
            # Required. Target for iso and source for components
            BIN="$OPTARG"
            ;;
        d)
            # Optional. directs script to make a debug iso instead of a production iso.
            debug="$OPTARG"
            ;;
        *)

            usage
            ;;
    esac
done

shift $((OPTIND-1))

# check there were no extra args and the required ones are set
if [ ! -z "$*" -o -z "$package" -o -z "${BIN}" ]; then
    usage
fi

#################################################################
# Above: arg parsing and setup
# Below: the image authoring
#################################################################

PKGDIR=$(mktemp -d)

unpack $package $PKGDIR

#selecting the init script as our entry point.
if [ -v debug ]; then
    export ISONAME="bootstrap-debug.iso"
    cp ${DIR}/bootstrap/bootstrap.debug $(rootfs_dir $PKGDIR)/bin/bootstrap
    cp ${BIN}/rpctool $(rootfs_dir $PKGDIR)/sbin/
else
    export ISONAME="bootstrap.iso"
    cp ${DIR}/bootstrap/bootstrap $(rootfs_dir $PKGDIR)/bin/bootstrap
fi

# copy in our components
cp ${BIN}/tether-linux $(rootfs_dir $PKGDIR)/bin/tether
cp ${BIN}/unpack $(rootfs_dir $PKGDIR)/bin/unpack

# kick off our components at boot time
mkdir -p $(rootfs_dir $PKGDIR)/etc/systemd/system/vic.target.wants
cp ${DIR}/bootstrap/tether.service $(rootfs_dir $PKGDIR)/etc/systemd/system/
cp ${DIR}/appliance/vic.target $(rootfs_dir $PKGDIR)/etc/systemd/system/
ln -s /etc/systemd/system/tether.service $(rootfs_dir $PKGDIR)/etc/systemd/system/vic.target.wants/
ln -sf /etc/systemd/system/vic.target $(rootfs_dir $PKGDIR)/etc/systemd/system/default.target

# disable networkd given we manage the link state directly
rm -f $(rootfs_dir $PKGDIR)/etc/systemd/system/multi-user.target.wants/systemd-networkd.service
rm -f $(rootfs_dir $PKGDIR)/etc/systemd/system/multi-user.target.wants/systemd-resolved.service
rm -f $(rootfs_dir $PKGDIR)/etc/systemd/system/sockets.target.wants/systemd-networkd.socket

# do not use the systemd dhcp client
rm -f $(rootfs_dir $PKGDIR)/etc/systemd/network/*
cp ${DIR}/base/no-dhcp.network $(rootfs_dir $PKGDIR)/etc/systemd/network/

# compute the size of the target tempfs,
# the list of directories/files in ${tempfs_target_list} should
# match the directories/files that are actually copied into tempfs
# by the script isos/bootstrap/bootstrap
target_list=$(rootfs_prepend $PKGDIR "${tempfs_target_list[@]}")
size=$(du -m --total ${target_list} | tail -1 | cut -f 1)
# 20% overhead should give a little more than 80M for stripped binaries
overhead=$(( size / 5 ))
size=$(( size + overhead ))
echo Total tempfs size: ${size}
# save the list of directories/files, for future usage
echo "${tempfs_target_list[@]}" > $(rootfs_dir $PKGDIR)/.tempfs_list
echo ${size} > $(rootfs_dir $PKGDIR)/.tempfs_size

generate_iso $PKGDIR $BIN/$ISONAME /lib/systemd/systemd
