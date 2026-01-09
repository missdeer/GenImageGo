#!/bin/bash
# Check current CPU type
CPU_TYPE=$(uname -m)

if [ "$CPU_TYPE" = "arm64" ]; then
  # Apple Silicon: build for x86_64
  CURARCH="arm64"
  ARCH="amd64"
else
  # Intel: build for aarch64
  CURARCH="amd64"
  ARCH="arm64"
fi

env GOARCH=$CURARCH go build -ldflags="-s -w" -o genImage-darwin-$CURARCH
env GOARCH=$ARCH go build -ldflags="-s -w" -o genImage-darwin-$ARCH
lipo -create -output genImage genImage-darwin-$CURARCH genImage-darwin-$ARCH
rm genImage-darwin-$CURARCH genImage-darwin-$ARCH
