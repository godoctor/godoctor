#!/bin/bash

# Vendoring of third-party packages for the Go Doctor

if [ `dirname $0` != '.' ]; then
	echo "vendor.sh must be run with releng as the current directory"
	exit 1
fi

# Change to the root of the godoctor repository
cd ..

FILE=`pwd`/internal/versions.txt

echo "Logging versions to $FILE..."
date >$FILE
for pkg in golang.org/x/tools github.com/cheggaaa/pb github.com/willf/bitset; do
	pushd . >/dev/null
	cd internal/$pkg
	echo "" >>$FILE
	echo "$pkg" >>$FILE
	git remote -v | head -1 >>$FILE
	git log --pretty=format:'%H %d %s' -1 >>$FILE
	echo "" >>$FILE
	popd >/dev/null
done

echo "Removing unused portions of go.tools..."
pushd . >/dev/null
cd internal/golang.org/x/tools && rm -rf blog cmd cover dashboard godoc imports oracle playground present refactor codereview.cfg go/callgraph go/gccgoimporter go/importer go/pointer go/vcs
popd >/dev/null

echo "Removing tests from third-party packages..."
find internal -iname '*_test.go' -delete

echo "Rewriting import paths in Go Doctor and third-party sources..."
find . -iname '*.go' -exec sed -e 's/"golang.org\/x\//"github.com\/godoctor\/godoctor\/internal\/golang.org\/x\//g' -i '' '{}' ';'
find . -iname '*.go' -exec sed -e 's/"github.com\/cheggaaa\//"github.com\/godoctor\/godoctor\/internal\/github.com\/cheggaaa\//g' -i '' '{}' ';'
find . -iname '*.go' -exec sed -e 's/"github.com\/willf\//"github.com\/godoctor\/godoctor\/internal\/github.com\/willf\//g' -i '' '{}' ';'

echo "DONE"
exit 0
