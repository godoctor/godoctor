// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains the command line interface for Go refactoring.

package main

//TODO(reed): this is getting rather crufty... go read other go CLI's.
// mainly there's prints everywhere when in reality these should all
// get shoved through one function

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang-refactoring.org/go-doctor/doctor"
)

var (
	formatFlag = flag.String("format", "plain",
		"output in 'plain' or 'json'")

	helpFlag = flag.Bool("h", false,
		"prints usage")

	diffFlag = flag.Bool("d", false,
		"get diff of all files affected by given refactoring")

	listFlag = flag.Bool("l", false,
		"list all possible refactorings")

	paramsFlag = flag.Bool("p", false,
		"get description of parameters for given refactoring")

	posFlag = flag.String("pos", "0,0:0,0",
		"line, col offset usually necessary, e.g. -pos=5,11:5,11")

	//TODO (reed) not sure if this actually works
	scopeFlag = flag.String("scope", "",
		"give a scope (package), e.g. -scope=code.google.com/p/go.tools/")

	writeFlag = flag.Bool("w", false,
		"write the refactored files in place")

	//useful for JSON I'm thinking
	skipLogFlag = flag.Bool("e", false,
		"write results even if log, dangerous")
)

func usage() {
	fmt.Fprintf(os.Stderr,
		`usage of `+os.Args[0]+`:`+"\n"+
			os.Args[0]+` [<flag> ...] <file> <refactoring> <args> ...

The <refactoring> may be one of:
%v

The <flag> arguments are

`,
		func() (s string) {
			for key, _ := range doctor.AllRefactorings() {
				s += "\n  " + key
			}
			return
		}())
	flag.PrintDefaults()
	fmt.Printf(`
<args> are <refactoring> specific and must be provided in order
for a <refactoring> to occur. To see the <args> for a <refactoring> do:

` + os.Args[0] + ` -p <refactoring>` + "\n")
}

type Response struct {
	Reply string
	Json  map[string]interface{}
	Plain []string
}

func (r Response) String() string {
	var s string
	switch *formatFlag {
	case "plain":
		for i, p := range r.Plain {
			s += p
			if i != len(r.Plain)-1 {
				s += "\n"
			}
		}
	case "json":
		r.Json["reply"] = r.Reply
		b, err := json.MarshalIndent(r.Json, "", "\t")
		s = string(b)
		if err != nil {
			s = ""
		}
	default:
		return "invalid -format flag"
	}
	return s
}

//TODO(reed) -comments to change comments (if a thing?)
//TODO(reed) scope... done?
//TODO(reed) handle errors better (JSON-wise, especially the not log stuff)
//TODO(reed) take byte offsets AND line:col
//
//example query: go-doctor -pos=11,8:11,8 someFile.go rename newName
func main() {
	err := attempt()
	if err != nil {
		r := Response{"Error", map[string]interface{}{"message": err.Error()}, []string{err.Error()}}
		fmt.Fprintf(os.Stderr, "%s\n", r)
	}
}

//TODO(reed) usage() in JSON
func attempt() error {
	flag.Parse()
	args := flag.Args()

	if *helpFlag {
		usage()
		return nil
	}

	if *listFlag {
		printAllRefactorings(*formatFlag)
		return nil
	}

	if len(args) == 0 {
		if *paramsFlag {
			return fmt.Errorf("no refactoring given to parameterize")
		}
		usage()
		return nil
	}

	var filename, src string

	r := doctor.GetRefactoring(args[0])

	if *paramsFlag {
		printRefactoringParams(r)
		return nil
	}

	//no file given (assume stdin), e.g. go-doctor refactor params...
	if r != nil && len(args) > 0 {
		args = args[1:]
		stat, err := os.Stdin.Stat()
		if err != nil {
			return err
		}
		if stat.Size() < 1 {
			return fmt.Errorf("no filename given and no input given on stdin")
		}
		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		src = string(bytes)
		filename = os.Stdin.Name()
	} else { //file given, e.g. go-doctor file refactor params...
		filename = args[0]
		r = doctor.GetRefactoring(args[1])
		args = args[2:]
	}

	//do the refactoring
	//TODO(reed): params much?
	l, es, err := query(filename, src, args, r, *posFlag, *scopeFlag)
	if err != nil {
		return err
	}

	//map[filename]output
	//where: output == diff || updated file
	changes := make(map[string][]byte)

	if l.ContainsErrors() && !*skipLogFlag {
		printResults(r.Name(), l, changes)
		return nil
	}

	if *diffFlag {
		//compute diff for each file changed
		for f, e := range es {
			var p *doctor.Patch
			var err error
			if src != "" {
				// TODO(reed): I suppose passing on stdin we can trust
				// that len(es) == 1, but I'm skeptical...
				// if scope is passed then multiple files could be effected.
				p, err = e.CreatePatch(strings.NewReader(src))
			} else {
				p, err = doctor.CreatePatchForFile(e, f)
			}
			if err != nil {
				return err
			}

			var b bytes.Buffer
			p.Write(f, f, &b)
			changes[f] = b.Bytes()
		}
		printResults(r.Name(), l, changes)
		return nil
	}

	//write all edits out to changes; something to work with
	for file, _ := range es {
		if src != "" {
			str, err := doctor.ApplyToString(es[file], src)
			if err != nil {
				return err
			}
			changes[file] = []byte(str)
		} else {
			changes[file], err = doctor.ApplyToFile(es[file], file)
			if err != nil {
				return err
			}
		}
	}

	//write changes to their file and exit
	if *writeFlag {
		if *diffFlag || *formatFlag != "plain" {
			return fmt.Errorf("cannot write files with json or diff flags, try again")
		}
		for file, change := range changes {
			if err := ioutil.WriteFile(file, change, 0); err != nil {
				return err
			}
		}
		return nil

	}

	// At this point changes has updated files and user did not give write flag,
	// so print refactored files.
	printResults(r.Name(), l, changes)
	return nil
}

func printResults(refactoring string, l *doctor.Log, changes map[string][]byte) {
	c := make(map[string]string)
	var contents []string

	for file, change := range changes {
		c[file] = string(change)
		contents = append(contents, string(change))
	}
	r := make(map[string]interface{})
	r["log"] = l
	r["name"] = refactoring
	r["changes"] = c
	repl := Response{"OK", r, contents}
	fmt.Printf("%s\n", repl)
}

func printRefactoringParams(r doctor.Refactoring) {
	resp := Response{"OK",
		map[string]interface{}{"params": r.GetParams()},
		r.GetParams(),
	}
	fmt.Printf("%s\n", resp)
}

func printAllRefactorings(format string) {
	var names []string
	for name, _ := range doctor.AllRefactorings() {
		names = append(names, name)
	}

	info := make(map[string]interface{})
	info["refactorings"] = names

	r := Response{"OK", info, names}
	fmt.Printf("%s\n", r)
}

// e.g. 302,6
func parseLineCol(linecol string) (int, int) {
	lc := strings.Split(linecol, ",")
	if l, err := strconv.ParseInt(lc[0], 10, 32); err == nil {
		if c, err := strconv.ParseInt(lc[1], 10, 32); err == nil {
			return int(l), int(c)
		}
	}

	return -1, -1
}

// e.g. pos=3,6:3,9
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

func parseScopes(scope string) []string {
	return strings.Split(scope, ",")
}

//TODO (jeff) figure out how to deal with scope/mainFile/2nd arg to SetSelection
// Anyway I think we need to know which file the main function is in,
// so I made that the second arg to SetSelection -- confirm with Alan
//TODO(reed): what jeff said, currently prints nothing if scope != nil
//
// This will do all of the configuration and execution for
// a refactoring (@op), returning the edits to be made and log.
// For use with the CLI, but have at it.
//
func query(file string, src string, args []string, r doctor.Refactoring, pos string, scope string) (*doctor.Log, map[string]doctor.EditSet, error) {
	if r == nil {
		return nil, nil, fmt.Errorf("invalid refactoring")
	}

	ts, err := parsePositionToTextSelection(pos)
	if err != nil {
		return nil, nil, err
	}

	s := parseScopes(scope)

	ts.Filename, err = filepath.Abs(file)
	if err != nil {
		return nil, nil, err
	}

	if scope == "" {
		s = []string{ts.Filename}
	}

	// TODO these 3 all return bool, but get checked in log. Not sure if
	// need a change here or not. Maybe move this entire function to main.go
	if !r.SetSelection(ts, s, src) {
		return nil, nil, fmt.Errorf("unable to set selection for %s at %s", file, pos)
	}
	if !r.Configure(args) {
		return nil, nil, fmt.Errorf("unable to configure refactoring with args %s", args)
	}
	r.Run()
	l, e := r.GetResult()
	return l, e, nil
}
