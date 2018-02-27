#!/bin/bash

set -e

SCRIPT_NAME=$(basename "$0")
DIR=$(cd "$(dirname "$0")" && pwd)
ROOT_FOLDER="$DIR/.."
PUBLISH_DIR=$ROOT_FOLDER/target/publish
TARGET_NAME=web-rust
IMAGE_NAME=web-rust
IMAGE_VERSION=latest
BUILD_RELEASE=true
SOURCE_RELEASE_DIR=$ROOT_FOLDER/target/release
SOURCE_DEBUG_DIR=$ROOT_FOLDER/target/debug
SOURCE_DIR=$SOURCE_RELEASE_DIR

usage()
{
    echo "$SCRIPT_NAME [options]"
    echo "Note: You might have to run this as root or sudo."
    echo ""
    echo "options"
    echo " -i, --image-name     Image name (default: web-rust)"
    echo " -v, --image-version  Docker Image Version (default: latest)"
    echo " -r, --build-release  Build release configuration - true|false (default: true)"
    exit 1;
}

process_args()
{
    save_next_arg=0
    for arg in "$@"
    do
        if [ $save_next_arg -eq 1 ]; then
            IMAGE_NAME="$arg"
            save_next_arg=0
        elif [ $save_next_arg -eq 2 ]; then
            IMAGE_VERSION="$arg"
            save_next_arg=0
        elif [ $save_next_arg -eq 3 ]; then
            BUILD_RELEASE="$arg"
            save_next_arg=0
        else
            case "$arg" in
                "-h" | "--help" ) usage;;
                "-i" | "--image-name" ) save_next_arg=1;;
                "-v" | "--image-version" ) save_next_arg=2;;
                "-r" | "--build-release" ) save_next_arg=3;;
                * ) usage;;
            esac
        fi
    done
}

# process command line args
process_args "$@"

# build bits
if [ "$BUILD_RELEASE" == "true" ]; then
    cargo build --release
else
    SOURCE_DIR=$SOURCE_DEBUG_DIR
    cargo build
fi

# copy release binary & Dockerfile to a "publish" folder
rm -rf "$PUBLISH_DIR"
mkdir -p "$PUBLISH_DIR"
cp "$ROOT_FOLDER/Dockerfile" "$PUBLISH_DIR"
cp "$SOURCE_DIR/$TARGET_NAME" "$PUBLISH_DIR"

# build the Docker image
pushd "$PUBLISH_DIR"
docker build -t "$IMAGE_NAME":"$IMAGE_VERSION" .
popd
