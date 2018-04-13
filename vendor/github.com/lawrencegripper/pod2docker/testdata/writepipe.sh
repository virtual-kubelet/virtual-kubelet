#!/bin/bash
set -e

pipe=/testdata/testpipe

if [[ ! -p $pipe ]]; then
    echo "Reader not running"
    exit 1
fi


if [[ "$1" ]]; then
    echo "$1" >$pipe
else
    echo "Hello from $$" >$pipe
fi

echo "Wrote hello to pipe..."
