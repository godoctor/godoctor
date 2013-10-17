#!/bin/bash
#
# Refactoring test driver
#

# Compile and install the go-doctor binary
go install
if [ $? -ne 0 ]; then
	exit $?
fi
doctor=$GOPATH/bin/go-doctor

# Then run it on projects from testdata/*
for dir in testdata/*/*
do
	mkdir temp
	echo "TEST: $dir"
	cp -R $dir temp/
	cd temp/`basename $dir`
	export GOPATH=`pwd`
	$doctor --runtests
	RESULT=$?
	cd ../..
	rm -rf temp
	if [ $RESULT -ne 0 ]; then
		echo "FAIL"
		exit $RESULT
       	fi
done
echo "PASS"
exit 0
