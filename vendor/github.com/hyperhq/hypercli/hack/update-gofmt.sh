#!/bin/bash
find . -name "*.go" | grep -v Godeps |grep -v vendor | xargs gofmt -s -w
