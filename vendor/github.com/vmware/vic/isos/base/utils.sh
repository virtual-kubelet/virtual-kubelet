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


# utility functions for staged authoring of ISOs
[ -n "$DEBUG" ] && set -x
BASE_DIR=$(dirname $(readlink -f "$BASH_SOURCE"))

if [ -z ${BUILD_NUMBER+x} ]; then
  BUILD_NUMBER=0
fi

VERSION=`git describe --abbrev=0 --tags`-${BUILD_NUMBER}-`git rev-parse --short HEAD`

# initialize a directory with the assumptions we make for authoring
# 1: target directory
initialize_bundle() {
    mkdir -p $1

    # we copy the xorriso config template during init as it's part of the base directory
    # - variable replacement occurs during generation step however
    cp $BASE_DIR/xorriso-options.cfg $1/xorriso-options.cfg

    mkdir -p $1/rootfs/var/lib/rpm $1/bootfs/boot

    rpm --root=$1/rootfs --initdb
    cp -a $BASE_DIR/isolinux $1/bootfs/boot/isolinux
}

# unpackage working ISO filesystem bundle
# args:
# 1: package (tar archive) - created by pack()
# 2: directory to unpack to
unpack() {
    mkdir -p $2 || {
        echo "Unable to create target directory $2 for unpacking: $?" 1>&2
        return 1
    }

    tar -C $2 -xf $1 || {
        echo "Error extracting package archive $1: $?" 1>&2
        return 2
    }

    # record the correct file ownerships and permissions if we cannot restore them
    if [ "$(id -u)" != "0" ]; then
        # for now we're just going to fail when this is run as non-root
        echo "Unable to preserve ownership or permissions - run as root" 1>&2
        return 3

        # Leaving this in here for later reference - successfully restored permissions at
        # boot time via a manifest and systemd unit but want to try to do so during build
        # time if possible
        echo "Storing correct file ownership and permissions restoration" 1>&2

        # we need to chain these permission files, because when the archive is retarred
        # we can no longer rely on tar tvf to supply the correct permissions.
        # FILO because repeated non-superuser unpacks/pack cycles will trample attrs otherwise
        if [ -e $2/tar-attr.cfg ]; then
            mv $2/tar-attr.cfg $2/tar-attr.cfg~
        fi
        tar_attr_to_cmd $1 rootfs > $2/tar-attr.cfg || {
            echo "Failed to preserve file owner and permissions - run as root to avoid this step: $?" 1>&2
            return 4
        }

        # make those FI options, LO in the file
        if [ -e $2/tar-attr.cfg~ ]; then
            cat $2/tar-attr.cfg~ >> $2/tar-attr.cfg
            rm -f $2/tar-attr.cfg~
        fi
    elif [ -e $2/tar-attr.cfg ]; then
        # restore the recorded attributes
        ( cd $2/rootfs && . ../tar-attr.cfg ) || {
            echo "Failed to restore file permissions from manifest: $?" 1>&2
            return 5
        }
    fi
}

# package up bundle
# 1: bundle base directory
# 2: target package (tgz)
pack() {
    #subshell so we don't end up with ./ leading all names
    out=$(readlink -f $2)
    (
        cd $1
        tar -zcf $out rootfs bootfs xorriso* || {
            echo "Failed to package bundle directory: $?" 1>&2
            return 1
        }
    )

    if [ -z "$DEBUG" ]; then
        rm -fr $1
    fi
}


# turn the permissions and owner/group info into xorriso options
# 1: the archive to process
# 2: the subdir in the archive to restrict output to
tar_attr_to_xorriso() {
    tar --numeric-owner -tvf $1 "$2" | awk -v prefix="$2" '
function convertId(id, type)
{
    idcmd="id -" type " "id

    idcmd | getline nid
    close(idcmd)

    return nid
}

function txt2octal(txt)
{
    # this is used to convert between text perms and octal
    v["r1"]=400; v["w2"]=200; v["x3"]=100; v["s3"]=4100; v["S3"]=4000
    v["r4"]=40 ; v["w5"]=20 ; v["x6"]=10 ; v["s6"]=2010; v["S6"]=2000
    v["r7"]=4  ; v["w8"]=2  ; v["x9"]=1  ; v["t9"]=1001; v["T9"]=1000

    val=0
    for (i=1; i<=9; i++)
        val=val+v[substr(txt, i+1, 1)i]

    return val
}

BEGIN {
}
/^[^l]/ {
    # assemble the permissions mdoe
    val=txt2octal($0)

    # make our commands relative
    sub(prefix, "." , $6)

    # translate to numeric ids from textual
    split($2, owner, "/")
    uid=owner[1]
    gid=owner[2]

    # convert to numeric
    # uid=convertId(uid,"u")
    # gid=convertId(gid,"g")

    chown[uid]=chown[uid]" "$6
    chgrp[gid]=chgrp[gid]" "$6
    chmod[val]=chmod[val]" "$6
}
END {
    for (uid in chown)
        print "chown", uid, chown[uid]

    for (gid in chgrp)
        print "chgrp", gid, chgrp[gid]

    for (mode in chmod)
        printf "chmod %4d %s\n", mode, chmod[mode]
}'
    return $?
}

# Helper to ensure, if possible, that the specified packages are installed
# ...: space separted list of packages
ensure_apt_packages() {
    local install

    # ensure we've got the utils we need
    for pkg in "$@"; do
        dpkg -s $pkg >/dev/null 2>&1 || install="$install $pkg"
    done

    if [ -n "$install" ]; then
        if [ "$(id -u)" != "0" ]; then
            echo "Need to install packages - rerun as root" 1>&2
            echo "packages: $install" 1>&2
            return 1
        fi

        # try without update first
        echo "Installing necessary packages: $install"
        apt-get -y install $install >/dev/null 2>&1 || {
            (apt-get update && apt-get -y install $install) || {
                echo "Failed to install $install packages: $?" 1>&2
                return 1
            }
        }
    fi
}

# build an ISO from the specified bundle directory.
# 1: bundle base directory
# 2: output file for ISO image - stdio:/dev/fd/1 can be used for stdout
# 3: init binary to use
generate_iso() {
    [ -n "$3" ] || {
        echo "Init binary must be specified to generate_iso" 1>&2
        return 1
    }

    ensure_apt_packages cpio xorriso || {
        echo "cpio and xorriso packages must be installed for ISO authoring: $?" 1>&2
        return 1
    }

    out=$(readlink -f $2)
    # subshell to avoid changing directory for invoker in failure cases
    (
        # operate relative to the package
        cd $1

        test -r bootfs/boot/isolinux/isolinux.bin -a -w bootfs/boot/isolinux/isolinux.cfg || {
            echo "isolinux files must exist in $1/boot/isolinux: $?" 1>&2
            return 2
        }

        # ensure the target init exists
        test -x rootfs/$3 || {
            echo "Specified init ($3) does not exist or is not executable: $?" 1>&2
            return 3
        }
        # set the init binary in isolinux.cfg
        sed -i -e "s|^#\(\s*append rdinit\)=_INIT_BINARY_|\1=$3|" bootfs/boot/isolinux/isolinux.cfg || {
            echo "Unable to update rdinit entry in isolinux.cfg: $?" 1>&2
            return 4
        }

        # create the initramfs archive - subshell to avoid changing directory
        echo "Constructing initramfs archive"
        ( cd rootfs && find | cpio -o -H newc | gzip --fast ) > bootfs/boot/core.gz || {
            echo "Failed to package root filesystem from $1/rootfs: $?" 1>&2
            return 5
        }

        echo "Embedding build version ${VERSION} (use BUILD_NUMBER environment variable to override)"
        sed -i -e "s/\${VERSION}/${VERSION}/" xorriso-options.cfg

        # deleting the file first seems to be necessary in some cases
        rm -f "$out"

        # generate the ISO and write it to $ISOOUT
        xorriso -dev "$out" -options_from_file xorriso-options.cfg || {
            echo "Failed to generate ISO file from package: $?" 1>&2
            return 6
        }
    )

    return
}


# Support use of yum cached packages with installroot
# This has been written to use getopts to:
# a. allow the cache to be optional
# b. as a reference for other functions
yum_cached() {
    usage() { echo "Usage: yum_cached [-c yum-cache(tgz)] [-u (update cache if present)] -p package-dir <options>" 1>&2; }

    # must ensure OPTIND is local, along with any set by processing
    local OPTIND flag cache update INSTALLROOT cmds
    while getopts "c:up:a:" flag; do
        case $flag in
            c)
            # Optional. Cache name (tgz)
            cache="$OPTARG"
            ;;

            u)
            # Optional. Update cache after running command
            update="true"
            ;;

            p)
            # Required. Package directory
            PKGDIR="$OPTARG"
            INSTALLROOT=$(rootfs_dir $PKGDIR)
            ;;

            *)
            usage
            return 1
            ;;
        esac
    done
    shift $((OPTIND-1))

    cmds="$*"

    # check there were no extra args and the required ones are set to sane values
    [ -e "$PKGDIR" ] || {
        echo "Specified package directory must exist" 1>&2
        return 1
    }

    # bundle specific - if we're cleaning the cache and we want it all gone
    # $1 because of the shift after getopts
    if [ "$1" == "clean" -a "$2" == "all" ]; then
        rm -fr ${INSTALLROOT}/var/cache/yum/*
    else
        # do this before we bother unpacking the cache
        ensure_apt_packages yum || {
            echo "cpio and xorriso packages must be installed for ISO authoring: $?" 1>&2
            return 2
        }

        # unpack cache
        if [ -n "${cache}" -a -e "${cache}" ]; then
            echo "Unpacking yum cache into ${INSTALLROOT}"

            tar -C ${INSTALLROOT} -zxf $cache || {
                echo "Unpacking yum cache $cache failed: $?" 1>&2
                return 3
            }
        fi

        /usr/bin/yum --installroot $INSTALLROOT $ACTION $cmds || {
            echo "Error while running yum command \"$cmds\": $?" 1>&2
            return 4
        }
    fi

    # repack cache
    if [ -n "$update" -a -n "${cache}" -a -d ${INSTALLROOT}/var/cache/yum ]; then
        tar -C ${INSTALLROOT} -zcf $cache var/cache/yum
    fi
}

# Runs a command in the rootfs of the specified bundle. This prevents callers from needing
# to know about internal bundle structure
# 1: bundle directory
# ...: command and args
rootfs_cmd() {
    (
        cd $1/rootfs || {
            echo "Specified directory $1 doesn't contain expected rootfs directory" 1>&2
            return 1
        }

        shift 1
        cmd=$1
        shift 1

        $cmd "$@" || return $?
    )
}

# Echos the full path of the root filesystem, given the bundle directory
# 1: bundle directory
rootfs_dir() {
    echo $1/rootfs
}

# Echos the full path of the boot filesystem, given the bundle directory
# 1: bundle directory
bootfs_dir() {
    echo $1/bootfs
}

rootfs_prepend() {
    list=("$@")
    rootfs=${list[0]}
    for ((index = 1; index < ${#list[@]}; ++index)); do
        out_index=$(( ${index} - 1 ))
        out_list[${out_index}]="$(rootfs_dir ${rootfs})${list[${index}]}"
    done
    echo ${out_list[@]}
}
