#!/bin/bash
#
# Refactoring test driver
#
# Usage: ./runtests.sh [filter]
#
# If filter is present, every test's name is compared against the filter using
#     "echo testname | grep filter" 
# and only tests whose names match are run.
#
# Examples:
#     ./runtests.sh
#     ./runtests.sh rename
#     ./runtests.sh rename/001
#

clear

# Compile and install the go-doctor binary
echo 'Compiling go-doctor...'
go install
if [ $? -ne 0 ]; then
	exit $?
fi
doctor=$GOPATH/bin/go-doctor

# Then run it on projects from testdata/*
echo 'Done compiling.  Running tests...'
rm -rf temp
for dir in testdata/*/*
do
	echo $dir | grep "$1" >/dev/null 2>&1
	RESULT=$?
	if [ $RESULT -eq 0 ]; then
		mkdir temp
		echo ""
		echo "TEST: $dir"
		cp -R $dir temp/
		cd temp/`basename $dir`
		export GOPATH=`pwd`
		$doctor -runtests=true
		RESULT=$?
		cd ../..
		rm -rf temp
		if [ $RESULT -ne 0 ]; then
			echo "FAIL"
			exit $RESULT
	       	fi
       	fi
done
echo "PASS"
exit 0
