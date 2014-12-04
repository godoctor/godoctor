#!/bin/bash

VERSION=0.1
PLATFORMS="darwin/386 darwin/amd64 linux/386 linux/amd64 windows/386 windows/amd64"
# Omitted: freebsd/386 freebsd/amd64

echo "Creating man page..."
go build \
  -ldflags "-X main.version ${VERSION}" \
  -o godoctor \
  `dirname $0`/../cmd/godoctor
if [ $? -ne 0 ]; then
  exit $?
fi
./godoctor -man >godoctor.1

for PLATFORM in $PLATFORMS; do
  GOOS=${PLATFORM%/*}
  GOARCH=${PLATFORM#*/}
  if [ "$GOOS" == "windows" ]; then
    SUFFIX=".exe"
  else
    SUFFIX=""
  fi

  echo "Building godoctor-${GOOS}-${GOARCH}${SUFFIX}..."

  DEST=`dirname $0`/godoctor-${VERSION}-${GOOS}-${GOARCH}
  rm -rf ${DEST} ${DEST}.zip

  GOOS=${GOOS} GOARCH=${GOARCH} \
    go build \
    -ldflags "-X main.version ${VERSION}" \
    -o godoctor${SUFFIX} \
    `dirname $0`/../cmd/godoctor
  if [ $? -eq 0 ]; then
    mkdir -p ${DEST}/bin
    mv godoctor${SUFFIX} ${DEST}/bin
    mkdir -p ${DEST}/man/man1
    cp godoctor.1 ${DEST}/man/man1
    zip -9r ${DEST}.zip ${DEST}
  fi
done

rm -f godoctor godoctor.1

echo "Computing MD5 and SHA-1 checksums..."
md5sum *.zip >md5sums.txt
sha1sum *.zip >sha1sums.txt

echo "Release packages are located in" `dirname $0`
exit 0
