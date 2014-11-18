#!/bin/bash

PLATFORMS="darwin/386 darwin/amd64 freebsd/386 freebsd/amd64 linux/386 linux/amd64 windows/386 windows/amd64"

for PLATFORM in $PLATFORMS; do
  GOOS=${PLATFORM%/*}
  GOARCH=${PLATFORM#*/}
  echo "building godoctor-${GOOS}-${GOARCH}..."
  GOOS=${GOOS} GOARCH=${GOARCH} go build -o pkg/godoctor-${GOOS}-${GOARCH} ./cmd/godoctor
done
echo "compiled binaries are located in ./pkg"
