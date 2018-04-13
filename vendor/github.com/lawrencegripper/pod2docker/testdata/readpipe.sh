#!/bin/bash
set -e
pipe=/testdata/testpipe

rm -f $pipe
mkfifo $pipe

if read line <$pipe; then
    echo $line
fi

echo "Reader exiting"