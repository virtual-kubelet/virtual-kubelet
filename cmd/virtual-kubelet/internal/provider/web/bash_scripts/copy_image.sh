#!/bin/sh
IMAGE_SOURCE=$1
IMAGE_DEST=$2

if test -z "$IMAGE_SOURCE" 
then
      echo "IMAGE_SOURCE is empty"
      exit 1
fi

if test -z "$IMAGE_DEST" 
then
      echo "IMAGE_DEST is empty"
      exit 1
fi

echo "IMAGE_SOURCE: " $IMAGE_SOURCE
echo "IMAGE_DEST: " $IMAGE_DEST

skopeo copy $IMAGE_SOURCE dir:$IMAGE_DEST