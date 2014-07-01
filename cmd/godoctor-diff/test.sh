#!/bin/bash

go install

# DIR=/Users/joverbey/Downloads/txt
if [ "$DIR" == "" ]
then
	echo "Usage: DIR=/path/to/some/directory ./test.sh"
	echo ""
	echo "This script will run dr-diff on each pair of files in the"
	echo "directory in the DIR environment variable, using patch to"
	echo "apply the resulting patch file and md5 to verify the file"
	echo "contents after the patch is applied."
	exit 1
fi

for file1 in $DIR/*.txt
do
	for file2 in $DIR/*.txt
	do
		# Use dr-diff to create patch
		echo "dr-diff $file1 $file2"
		dr-diff $file1 $file2 >patch.txt
		if [ $? -ne 0 ]
		then
			echo "FAIL"
			exit $?
		fi

		# Apply patch and compare to file2
		cp $file1 output.txt
		patch -p0 -s output.txt patch.txt
		EXPECTED=`md5 -q $file2`
		ACTUAL=`md5 -q output.txt`
		if [ "$EXPECTED" != "$ACTUAL" ]
		then
			echo "dr-diff failed: $file1 $file2"
			echo "Expected MD5: $EXPECTED"
			echo "Actual MD5: $ACTUAL"
			# Don't delete output.txt and patch.txt
			exit 99
		fi

		rm -f output.txt patch.txt
	done
done
