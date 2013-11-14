// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains the command line interface for Go refactoring.

// Contributors: Reed Allman, Josh Kane

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"golang-refactoring.org/go-doctor/doctor"
	"io/ioutil"
	"os"
	"os/exec"
)

var (
	formatFlag = flag.String("format", "plain",
		"Output in 'plain' or 'json', default: plain")

	helpFlag = flag.Bool("h", false,
		"Prints usage")

	diffFlag = flag.Bool("d", false,
		"Get diff of all files affected by given refactoring")

	listFlag = flag.Bool("l", false,
		"List all possible refactorings")

	paramsFlag = flag.Bool("p", false,
		"Get description of parameters for given refactoring")

	posFlag = flag.String("pos", "0,0:0,0",
		"Line, col offset usually necessary, e.g. -pos=5,11:5,11")

	//TODO (reed) need to understand this happening
	scopeFlag = flag.String("scope", "",
		"Give a scope (package), e.g. -scope=code.google.com/p/go.tools/")

	writeFlag = flag.Bool("w", false,
		"Write the refactored files in place")

	//useful for JSON I'm thinking
	skipLogFlag = flag.Bool("e", false,
		"Write results even if log, dangerous")
)

func usage() {
	//TODO figure out multi line strings and get back to me
	fmt.Printf(`Usage of %s:
  %s [<flag> ...] <file> <refactoring> <args> ...

  The <refactoring> may be one of:
%v

  The <flag> arguments are

`,
		os.Args[0], os.Args[0],
		//TODO yeahhhh slow down there chief
		func() (s string) {
			for key, _ := range doctor.AllRefactorings() {
				s += "\n  " + key
			}
			return
		}())
	flag.PrintDefaults()
	os.Exit(1)
}

//TODO (reed / josh)  -comments to change comments (if a thing?)
//TODO learn to func
//
//TODO (reed / josh) scope (importer? wait?)
//
//example query: go-doctor -pos=11,8:11,8 someFile.go rename newName
//TODO query (stdin): cat file.go | go-doctor -pos=11,8:11,8 rename newName
func main() {
	flag.Parse()
	args := flag.Args()

	if *helpFlag {
		usage()
	}

	//print all possible refactorings
	if *listFlag {
		//TODO eh not sure I like putting this in doctor
		doctor.PrintAllRefactorings(*formatFlag)
		os.Exit(0)
	}

	if len(args) == 0 {
		fmt.Errorf("Error: Refactoring required")
		usage()
	}

	r := doctor.GetRefactoring(args[0])
	var name string

	//no file given (assume stdin), e.g. go-doctor refactor params...
	//TODO make stdin and importer get along
	if r != nil {
		name = "temp"
		args = args[1:]
	} else {
		//file given, e.g. go-doctor file refactor params...
		r = doctor.GetRefactoring(args[1])
		name = args[0]
		args = args[2:]
	}

	//just return parameters for refactoring
	if *paramsFlag {
		doctor.PrintRefactoringParams(r, *formatFlag)
		os.Exit(0)
	}

	//do the refactoring
	l, es, err := doctor.Query(name, args, r, *posFlag, *scopeFlag)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//TODO fall through if json?
	if l.ContainsErrors() && !*skipLogFlag {
		fmt.Println(l)
		os.Exit(1)
	}

	changes := make(map[string][]byte)
	var buf bytes.Buffer

	//write all edits out to changes; something to work with
	for file, _ := range es.Edits() {
		if err := es.ApplyToFile(file, &buf); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		changes[file] = buf.Bytes()
	}

	if *writeFlag {
		//write changes to their file and exit
		for file, change := range changes {
			if err := ioutil.WriteFile(file, change, 0); err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
		}
		return
	}

	if *diffFlag {
		//compute diff for each
		for file, change := range changes {
			f, err := ioutil.TempFile("", "go-doctor")
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			//TODO make sure that we return, so this happens
			defer os.Remove(f.Name())
			defer f.Close()

			f.Write(change)
			diff, err := exec.Command("diff", "-u", file, f.Name()).CombinedOutput()
			if len(diff) > 0 {
				//diff exits with a non-zero status when the files don't match.
				//Ignore that failure as long as we get output.
				err = nil
			}
			if err != nil {
				fmt.Println(err)
				return
			}
			//put diff in changes instead of just changed file
			changes[file] = diff
		}
	}

	//at this point changes either has updated files or diff data
	//output what we have
	switch *formatFlag {
	case "plain":
		for file, change := range changes {
			//TODO show file name, piss off the unix gurus?
			fmt.Printf("%s:\n\n", file)
			fmt.Printf("%s\n", change)
		}
	case "json":
		//TODO figure out a better way, O(N) says so.
		//[]byte goes to base64 string in json
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
			return
		}
		fmt.Printf("%s\n", out)
	}
}
