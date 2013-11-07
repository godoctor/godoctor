package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go-doctor/doctor"
	"io/ioutil"
	"os"
)

var (
	formatFlag = flag.String("format", "plain",
		"Output in 'plain' or 'json', default: plain")

	runTestsFlag = flag.Bool("runtests", false,
		"(For internal use only)")

	paramsFlag = flag.Bool("p", false,
		"Get description of parameters for given refactoring")

	posFlag = flag.String("pos", "",
		"Line, col offset usually necessary, e.g. -pos=5,11:5,11")

	//TODO (reed) need to understand this happening
	scopeFlag = flag.String("scope", "",
		"Give a scope (package), e.g. -scope=code.google.com/p/go.tools/")

	writeFlag = flag.Bool("w", false, "Write the refactored files in place")

	//useful for JSON I'm thinking
	skipLogFlag = flag.Bool("e", false, "Write results even if log, dangerous")
)

func usage() {
	//TODO figure out multi line strings and get back to me
	fmt.Errorf(`Usage of %s:
  %s [<flag> ...] <file> <refactoring> <args> ...

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

//TODO (reed / josh) hash out the json thing
// -d for diff files, -comments to change comments (if a thing?)
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
		return
	}

	//this is TBD per refactoring
	if *posFlag == "" {
		//fmt.Errorf("Error: -pos required")
		//usage()
	}

	if len(args) == 0 {
		fmt.Errorf("Error: Refactoring required")
		usage()
	}

	r := doctor.GetRefactoring(args[0])
	var name string

	//no file given (assume stdin)
	//TODO CEPT THIS DOESN'T WORK MAN, thanks importer
	if r != nil {
		name = "temp"
		args = args[1:]
	} else {
		//file given
		r = doctor.GetRefactoring(args[1])
		name = args[0]
		args = args[2:]
	}

	if *paramsFlag {
		switch *formatFlag {
		case "plain":
			for _, p := range r.GetParams() {
				fmt.Println(p)
			}
		case "json":
			p, err := json.MarshalIndent(struct {
				Params []string `json:"params"`
			}{
				r.GetParams(),
			}, "", "\t")
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			fmt.Printf("%s\n", p)
		}
		os.Exit(0)
	}

	l, es := doctor.Query(name, args, r, *posFlag, *scopeFlag)
	if l.ContainsErrors() && !*skipLogFlag {
		fmt.Println(l)
		os.Exit(1)
	}

	changes := make(map[string][]byte)
	var buf bytes.Buffer

	//write all edits out to changes[filename]contents
	for file, _ := range es.Edits() {
		if err := es.ApplyToFile(file, &buf); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		changes[file] = buf.Bytes()
	}

	if *writeFlag {
		for file, change := range changes {
			if err := ioutil.WriteFile(file, change, 0); err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
		}
	} else {
		switch *formatFlag {
		case "plain":
			for file, change := range changes {
				//TODO show file name, for pissing off the unix gurus?
				fmt.Printf("%s:\n\n", file)
				fmt.Printf("%s\n", change)
			}
		case "json":
			//TODO figure out a better way, O(N) says so
			//bytes <-> string not so nice
			c := make(map[string]string)
			for file, change := range changes {
				c[file] = string(change[:])
			}
			out, err := json.MarshalIndent(struct {
				Name    string            `json:"name"`
				Log     *doctor.Log       `json:"log"`
				Changes map[string]string `json:"changes"`
			}{
				r.Name(),
				l,
				c,
			}, "", "\t")
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			fmt.Printf("%s\n", out)
		}
	}
}
