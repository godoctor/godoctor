package main

import (
	"flag"
	"fmt"
	"go-doctor/doctor"
	"io/ioutil"
	"os"
)

var (
	runTestsFlag = flag.Bool("runtests", false,
		"(For internal use only)")

	posFlag = flag.String("pos", "",
		"Filename and byte offset are necessary, e.g. -pos=foo.go:#500,#505")

	scopeFlag = flag.String("scope", "",
		"If you'd like, give a scope, e.g. -scope=code.google.com/p/go.tools/")

	writeFlag = flag.Bool("w", false, "Write the refactored files in place")

	skipLogFlag = flag.Bool("l", false, "Write results even if log, dangerous")
)

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

//TODO (reed / josh) handle -format=json,
// -d for diff files, -comments to change comments (if a thing?)
//e.g. be a lot like gofmt, which I've listed below
//
//TODO (reed / josh) scope (importer? wait?)
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

//example query: go-doctor -pos=11,8:11,8 someFile.go rename heloooooooo
func main() {
	flag.Parse()
	args := flag.Args()

	if *runTestsFlag == true {
		doctor.RunAllTests()
		os.Exit(0)
	}

	if *posFlag == "" {
		fmt.Errorf("Error: -pos required")
		usage()
	}

	if len(args) == 0 {
		fmt.Errorf("Error: Refactoring required")
		usage()
	}

	r := doctor.GetRefactoring(args[1])
	var name string

	//no file given (assume stdin)
	//TODO CEPT THIS DOESN'T WORK MAN, thanks importer
	if r == nil {
		f, err := ioutil.TempFile("", "go doctor")
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		defer os.Remove(f.Name())
		defer f.Close()
		//write stdin to file... TODO but don't
		input, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Println("helloo there")
		}
		err = ioutil.WriteFile(f.Name(), input, 0)
		if err != nil {
			fmt.Println("here we are")
		}

		name = f.Name()
		r = doctor.GetRefactoring(args[0])
		args = args[1:]
	} else {
		//file given
		name = args[0]
		args = args[2:]
	}

	l, es := doctor.Query(name, args, r, *posFlag, *scopeFlag)
	if l.ContainsErrors() && !*skipLogFlag {
		fmt.Println(l)
		os.Exit(1)
	}

	//TODO (reed) consider not doing the WriteTo..()
	//and ApplyTo..() methods b/c JSON will need one
	//and because EditSet's don't need to know by which means
	//to edit themselves, the driver should take care of it.
	//
	//Just write to []byte for each file, then do things
	//
	if *writeFlag {
		err := es.WriteToAllFiles()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		//for now this just prints to stdout
		err := es.ApplyToAllFiles(os.Stdout)

		if err != nil {
			os.Exit(1)
		}
	}

	//TODO different output handling

}
