// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains the command line interface for Go refactoring.

// Contributors: Reed Allman, Josh Kane

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"golang-refactoring.org/go-doctor/doctor"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
		printAllRefactorings(*formatFlag)
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
		printRefactoringParams(r, *formatFlag)
		os.Exit(0)
	}

	//do the refactoring
	l, es, err := query(name, args, r, *posFlag, *scopeFlag)

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

	//write all edits out to changes; something to work with
	for file, _ := range es {
		changes[file], err = doctor.ApplyToFile(es[file], file)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
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

//Figure out how much I like this...
func printRefactoringParams(r doctor.Refactoring, format string) {
	switch format {
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
}

func printAllRefactorings(format string) {
	var names []string
	for name, _ := range doctor.AllRefactorings() {
		names = append(names, name)
	}

	switch format {
	case "plain":
		for _, n := range names {
			fmt.Println(n)
		}
	case "json":
		p, err := json.MarshalIndent(struct {
			Refactorings []string `json:"refactorings"`
		}{
			names,
		}, "", "\t")
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		fmt.Printf("%s\n", p)
	}
}

//e.g. 302,6
func parseLineCol(linecol string) (int, int) {
	lc := strings.Split(linecol, ",")
	if l, err := strconv.ParseInt(lc[0], 10, 32); err == nil {
		if c, err := strconv.ParseInt(lc[1], 10, 32); err == nil {
			return int(l), int(c)
		}
	}

	return -1, -1
}

//pos=3,6:3,9
func parsePositionToTextSelection(pos string) (t doctor.TextSelection, err error) {
	args := strings.Split(pos, ":")

	if len(args) < 2 {
		err = fmt.Errorf("invalid -pos")
		return
	}

	sl, sc := parseLineCol(args[0])
	el, ec := parseLineCol(args[1])

	if sl < 0 || sc < 0 || el < 0 || ec < 0 {
		err = fmt.Errorf("invalid -pos line, col")
		return
	}

	t = doctor.TextSelection{StartLine: sl, StartCol: sc,
		EndLine: el, EndCol: ec}

	return
}

//TODO (reed / josh) scope here?
//TODO (jeff) I'm fairly sure I used scope wrong here...?
// Anyway I think we need to know which file the main function is in,
// so I made that the second arg to SetSelection -- confirm with Alan
//
//This will do all of the configuration and execution for
//a refactoring (@op), returning the edits to be made and log.
//For use with the CLI, but have at it.
//
func query(file string, args []string, r doctor.Refactoring, pos string, scope string) (*doctor.Log, map[string]doctor.EditSet, error) {
	if r == nil {
		return nil, nil, fmt.Errorf("Invalid refactoring")
	}

	ts, err := parsePositionToTextSelection(pos)
	if err != nil {
		return nil, nil, err
	}
	ts.Filename = file

	// TODO these 3 all return bool, but get checked in log. Not sure if
	// need a change here or not. Maybe move this entire function to main.go
	r.SetSelection(ts, scope)
	r.Configure(args)
	r.Run()
	e, l := r.GetResult()
	return e, l, nil
}
