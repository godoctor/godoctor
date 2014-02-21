#!/bin/bash

# currently local1.go only exists on reed's local, so don't worry about these passing
# I'm just getting sick of typing these :o

# TODO(reed): don't just dump these, assert something

go-doctor -h
go-doctor -l
go-doctor -l -format json
go-doctor -p  #error
go-doctor -p rename
go-doctor -p -format json rename
go-doctor -pos 11,6:11,6 local1.go rename supss
go-doctor -pos 11,6:11,6 -format json local1.go rename supss
go-doctor -pos 11,6:11,6 -d local1.go rename supss
go-doctor -pos 11,6:11,6 -d -format json local1.go rename supss
go-doctor -pos 11,6:11,6 -scope=`pwd` local1.go rename supss
go-doctor -pos 11,6:11,6 -w local1.go rename supss

