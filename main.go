package main

import (
	"flag"
	"fmt"
	"go-doctor/doctor"
	"os"
)

var posFlag = flag.String("pos", "",
	"Filename and byte offset are necessary, e.g. -pos=foo.go:#500,#505")

var scopeFlag = flag.String("scope", "",
	"If you'd like, give a scope, e.g. -scope=code.google.com/p/go.tools/")

func usage() {
	//TODO figure out multi line strings and get back to me
	fmt.Errorf(`Usage of %s:
  %s [<flag> ...] <refactoring> <args> ...

  The <refactoring> may be one of:

  %v

  The <flag> arguments are

  `,
		os.Args[0], os.Args[0],
		//TODO yeahhhh slow down there chief
		func() (s string) {
			for key, _ := range doctor.GetAllRefactorings() {
				s += key + "\n"
			}
			return
		}())
	flag.PrintDefaults()
	os.Exit(1)
}

//TODO (reed / josh) handle -w to write, -format=json, -? to skip log
// -d for diff files, -comments to change comments (if a thing?)
//e.g. be a lot like gofmt, which I've listed below
//
//TODO (reed / josh) scope (importer? wait a month?)
//
//HERE BE gofmt
//
//usage: gofmt [flags] [path ...]
//-comments=true: print comments
//-cpuprofile="": write cpu profile to this file
//-d=false: display diffs instead of rewriting files
//-e=false: report all errors (not just the first 10 on different lines)
//-l=false: list files whose formatting differs from gofmt's
//-r="": rewrite rule (e.g., 'a[b:len(a)] -> a[b:]')
//-s=false: simplify code
//-tabs=true: indent with tabs
//-tabwidth=8: tab width
//-w=false: write result to (source) file instead of stdout

//example query: go-doctor -pos=testdata/rename/001-local/local1.go:11,8:11,8 rename heloooooooo
func main() {
	flag.Parse()
	args := flag.Args()

	// FIXME: Modify main so this only happens if you pass --runtests
	// TODO: (reed) you sure?
	//doctor.RunAllTests()

	if *posFlag == "" {
		fmt.Errorf("Error: -pos required")
		usage()
	}

	if len(args) == 0 {
		fmt.Errorf("Error: Refactoring required")
		usage()
	}

	l, es := doctor.Query(args[1:], args[0], *posFlag, *scopeFlag)
	if l.ContainsErrors() {
		fmt.Println(l)
		os.Exit(1)
	}

	//for now this just prints to stdout
	fmt.Println(es.ApplyToAllFiles(os.Stdout))

	//TODO different output handling

}
