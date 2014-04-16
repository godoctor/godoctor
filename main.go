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
	formatFlag = flag.String("format", "plain", "output in 'plain' or 'json'")
	helpFlag   = flag.Bool("h", false, "prints usage")
	diffFlag   = flag.Bool("d", false, "get diff of all files affected by given refactoring")
	listFlag   = flag.Bool("l", false, "list all possible refactorings")
	paramsFlag = flag.Bool("p", false, "get description of parameters for given refactoring")
	posFlag    = flag.String("pos", "0,0:0,0", "line, col offset usually necessary, e.g. -pos=5,11:5,11")
	//TODO (reed) not sure if this actually works
	scopeFlag = flag.String("scope", "", "give a scope (package), e.g. -scope=code.google.com/p/go.tools/")
	writeFlag = flag.Bool("w", false, "write the refactored files in place")
	//useful for JSON I'm thinking
	skipLogFlag = flag.Bool("e", false, "write results even if log, dangerous")
)

func usage() {
	fmt.Fprintf(os.Stderr,
		`usage of `+os.Args[0]+`:

  `+os.Args[0]+` [<flag> ...] <file> <refactoring> <args> ...

The <refactoring> may be one of:
%v

<args> are <refactoring> specific and must be provided in order
for a <refactoring> to occur. To see the <args> for a <refactoring> do:

  `+os.Args[0]+` -p <refactoring>

The <flag> arguments are:

`,
		func() (s string) {
			for key, _ := range doctor.AllRefactorings() {
				s += "\n  " + key
			}
			return
		}())
	flag.PrintDefaults()
	os.Exit(2)
}

type Response struct {
	Reply string
	JSON  fields
	Plain []string
}

//this got real old
type fields map[string]interface{}

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
		r.JSON["reply"] = r.Reply
		b, err := json.MarshalIndent(r.JSON, "", "\t")
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
		r := Response{"Error", fields{"message": err.Error()}, []string{err.Error()}}
		fmt.Fprintf(os.Stderr, "%s\n", r)
		os.Exit(2)
	}
}

//TODO(reed) usage() in JSON
//TODO(reed) find weird flag combos that shouldn't work
func attempt() error {
	flag.Parse()
	args := flag.Args()

	//TODO(reed) are we [ever] going to have a default to run w/o any args or flags?
	if *helpFlag || flag.NFlag() == 0 {
		usage()
	}

	if *listFlag {
		printAllRefactorings(*formatFlag)
		return nil
	}

	if flag.NArg() == 0 {
		return fmt.Errorf("given flag requires args, see -h")
	}

	r := doctor.GetRefactoring(args[0])

	if *paramsFlag {
		if r == nil {
			return fmt.Errorf("no refactoring given to parameterize, see -h")
		}
		printRefactoringParams(r)
		return nil
	}

	var filename, src string
	// no file given (assume stdin), e.g. go-doctor refactor params...
	if r != nil {
		if stat, err := os.Stdin.Stat(); err != nil {
			return err
		} else if stat.Size() < 1 {
			return fmt.Errorf("no filename given and no input given on stdin, see -h")
		}
		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		src = string(bytes)
		filename = "main.go" // was os.Stdin.Name()
		args = args[1:]
	} else { // file given, e.g. go-doctor file refactor params...
		filename = args[0]
		r = doctor.GetRefactoring(args[1])
		args = args[2:]
	}
	// at this point args = refactoring's args, possibly none

	// TODO(reed): params much?
	// do the refactoring
	log, edits, err := query(filename, src, args, r, *posFlag, *scopeFlag)
	if err != nil {
		return err
	}

	// map[filename]output
	// where: output == diff || updated file
	changes := make(map[string][]byte)

	if log.ContainsErrors() && !*skipLogFlag {
		printResults(r.Description().Name, log, changes)
		return nil
	}

	if *diffFlag {
		// compute diff for each file changed
		for f, e := range edits {
			var p *doctor.Patch
			var err error
			if src != "" {
				// TODO(reed): I suppose passing on stdin we can trust
				// that len(edits) == 1, but I'm skeptical...
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
		printResults(r.Description().Name, log, changes)
		return nil
	}

	//write all edits out to new file contents in []byte; something to work with
	for file, _ := range edits {
		if src != "" {
			str, err := doctor.ApplyToString(edits[file], src)
			if err != nil {
				return err
			}
			changes[file] = []byte(str)
		} else {
			changes[file], err = doctor.ApplyToFile(edits[file], file)
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
	printResults(r.Description().Name, log, changes)
	return nil
}

func printResults(refactoring string, l *doctor.Log, changes map[string][]byte) {
	c := make(map[string]string)
	var contents []string

	for file, change := range changes {
		c[file] = string(change)
		contents = append(contents, string(change))
	}
	r := fields{
		"log":     l,
		"name":    refactoring,
		"changes": c,
	}
	repl := Response{"OK", r, contents}
	fmt.Printf("%s\n", repl)
}

func printRefactoringParams(r doctor.Refactoring) {
	resp := Response{"OK",
		fields{"params": r.Description().Params},
		r.Description().Params,
	}
	fmt.Printf("%s\n", resp)
}

func printAllRefactorings(format string) {
	var names []string
	for name, _ := range doctor.AllRefactorings() {
		names = append(names, name)
	}

	info := fields{
		"refactorings": names,
	}

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
		return nil, nil, fmt.Errorf("no refactoring given or in wrong place, see -h")
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

	var fs doctor.FileSystem
	if src != "" {
		fs = &doctor.VirtualFileSystem{}
		fs.CreateFile("main.go", src)
	} else {
		fs = &doctor.LocalFileSystem{}
	}

	config := &doctor.Config{
		FileSystem: fs,
		Scope:      s,
		Selection:  ts,
		Args:       args,
	}
	result := r.Run(config)
	fmt.Println(result.Log)
	return result.Log, result.Edits, nil
}
