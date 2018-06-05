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

# Build the appliance filesystem ontop of the base

# exit on failure and configure debug, include util functions
set -e && [ -n "$DEBUG" ] && set -x
DIR=$(dirname $(readlink -f "$0"))
. $DIR/base/utils.sh


function usage() {
echo "Usage: $0 -p staged-package(tgz) -b binary-dir" 1>&2
exit 1
}

while getopts "p:b:" flag
do
    case $flag in

        p)
            # Required. Package name
            PACKAGE="$OPTARG"
            ;;

        b)
            # Required. Target for iso and source for components
            BIN="$OPTARG"
            ;;

        *)
            usage
            ;;
    esac
done

shift $((OPTIND-1))

# check there were no extra args and the required ones are set
if [ ! -z "$*" -o -z "$PACKAGE" -o -z "${BIN}" ]; then
    usage
fi

PKGDIR=$(mktemp -d)

# unpackage base package
unpack $PACKAGE $PKGDIR

#################################################################
# Above: arg parsing and setup
# Below: the image authoring
#################################################################

# sysctl
cp ${DIR}/appliance/sysctl.conf $(rootfs_dir $PKGDIR)/etc/

## systemd configuration
# create systemd vic target
cp ${DIR}/appliance/vic.target $(rootfs_dir $PKGDIR)/etc/systemd/system/
cp ${DIR}/appliance/*.service $(rootfs_dir $PKGDIR)/etc/systemd/system/
cp ${DIR}/appliance/*-setup $(rootfs_dir $PKGDIR)/etc/systemd/scripts

mkdir -p $(rootfs_dir $PKGDIR)/etc/systemd/system/vic.target.wants
ln -s /etc/systemd/system/vic-init.service $(rootfs_dir $PKGDIR)/etc/systemd/system/vic.target.wants/
ln -s /etc/systemd/system/nat.service $(rootfs_dir $PKGDIR)/etc/systemd/system/vic.target.wants/
ln -s /etc/systemd/system/permissions.service $(rootfs_dir $PKGDIR)/etc/systemd/system/vic.target.wants/
ln -s /lib/systemd/system/multi-user.target $(rootfs_dir $PKGDIR)/etc/systemd/system/vic.target.wants/

# disable networkd given we manage the link state directly
rm -f $(rootfs_dir $PKGDIR)/etc/systemd/system/multi-user.target.wants/systemd-networkd.service
rm -f $(rootfs_dir $PKGDIR)/etc/systemd/system/sockets.target.wants/systemd-networkd.socket

# Disable time synching.  We'll use toolbox for this.
rm -f $(rootfs_dir $PKGDIR)/etc/systemd/system/sysinit.target.wants/systemd-timesyncd.service

# change the default systemd target to launch VIC
ln -sf /etc/systemd/system/vic.target $(rootfs_dir $PKGDIR)/etc/systemd/system/default.target

# do not use the systemd dhcp client
rm -f $(rootfs_dir $PKGDIR)/etc/systemd/network/*
cp ${DIR}/base/no-dhcp.network $(rootfs_dir $PKGDIR)/etc/systemd/network/

# do not use the default iptables rules - nat-setup supplants this
rm -f $(rootfs_dir $PKGDIR)/etc/systemd/network/*

#
# Set up component users
#

chroot $(rootfs_dir $PKGDIR) groupadd -g 1000 vicadmin
chroot $(rootfs_dir $PKGDIR) useradd -u 1000 -g 1000 -G systemd-journal -m -d /home/vicadmin -s /bin/false vicadmin

# Group vic should be used to run all VIC related services.
chroot $(rootfs_dir $PKGDIR) groupadd -g 1001 vic
chroot $(rootfs_dir $PKGDIR) usermod -a -G vic vicadmin

cp -R ${DIR}/vicadmin/* $(rootfs_dir $PKGDIR)/home/vicadmin
chown -R 1000:1000 $(rootfs_dir $PKGDIR)/home/vicadmin

# so vicadmin can read the system journal via journalctl
install -m 755 -d $(rootfs_dir $PKGDIR)/etc/tmpfiles.d
echo "m  /var/log/journal/%m/system.journal 2755 root systemd-journal - -" > $(rootfs_dir $PKGDIR)/etc/tmpfiles.d/systemd.conf

chroot $(rootfs_dir $PKGDIR) mkdir -p /var/run/lock
chroot $(rootfs_dir $PKGDIR) chmod 1777 /var/run/lock
chroot $(rootfs_dir $PKGDIR) touch /var/run/lock/logrotate_run.lock
chroot $(rootfs_dir $PKGDIR) chown root:vic /var/run/lock/logrotate_run.lock
chroot $(rootfs_dir $PKGDIR) chmod 0660 /var/run/lock/logrotate_run.lock

## main VIC components
# tether based init
cp ${BIN}/vic-init $(rootfs_dir $PKGDIR)/sbin/vic-init

cp ${BIN}/{docker-engine-server,port-layer-server,vicadmin} $(rootfs_dir $PKGDIR)/sbin/
cp ${BIN}/unpack $(rootfs_dir $PKGDIR)/bin/

## Generate the ISO
# Select systemd for our init process
generate_iso $PKGDIR $BIN/appliance.iso /lib/systemd/systemd
