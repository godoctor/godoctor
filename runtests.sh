#!/bin/bash
#
# Refactoring test driver
#

go install
if [ $? -ne 0 ]; then
	exit $?
fi

DOCTOR=$GOPATH/bin/go-doctor

for dir in testdata/*/*
do
	mkdir temp
	echo "TEST: $dir"
	cp -R $dir temp/
	cd temp/`basename $dir`
	export GOPATH=`pwd`
	$DOCTOR --runtests
	RESULT=$?
	cd ../..
	rm -rf temp
	if [ $RESULT -ne 0 ]; then
		echo FAIL
		exit $RESULT
       	fi
done
echo PASS
exit 0
