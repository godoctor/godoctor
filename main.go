// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains the command line interface for Go refactoring.
package main

// example query: go-doctor -pos=11,8:11,8 someFile.go rename newName

// TODO(reed): still not proud of this.
// TODO(reed) take byte offsets AND line:col

// TODO(jeff/robert): Support "put" command for Web demo

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
	"golang-refactoring.org/go-doctor/protocol"
)

var (
	formatFlag = flag.String("format", "plain", "output in 'plain' or 'json'")
	helpFlag   = flag.Bool("h", false, "prints usage")
	diffFlag   = flag.Bool("d", false, "get diff of all files affected by given refactoring")
	listFlag   = flag.Bool("l", false, "list all possible refactorings")
	paramsFlag = flag.Bool("p", false, "get description of parameters for given refactoring")
	posFlag    = flag.String("pos", "0,0:0,0", "line, col offset usually necessary, e.g. -pos=5,11:5,11")
	scopeFlag  = flag.String("scope", "", "give a scope (package), e.g. -scope=code.google.com/p/go.tools/")
	writeFlag  = flag.Bool("w", false, "write the refactored files in place")
	// need json usage
	jsonFlag = flag.Bool("json", false, "")
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

// TODO(reed) usage() in JSON
// TODO(reed/somebodyelse) find weird flag combos that shouldn't work
func main() {
	flag.Parse()
	args := flag.Args()

	// TODO(reed) are we [ever] going to have a default to run w/o any args or flags?
	if *helpFlag || flag.NFlag() == 0 {
		usage()
	}

	if *listFlag {
		printAllRefactorings(*formatFlag)
		return
	}

	if *jsonFlag {
		protocol.Run(args)
		return
	}

	if flag.NArg() == 0 {
		printError(fmt.Errorf("given flag requires args, see -h"))
	}

	r := doctor.GetRefactoring(args[0])

	if *paramsFlag {
		if r == nil {
			printError(fmt.Errorf("no refactoring given to parameterize, see -h"))
		}
		printRefactoringParams(r)
		return
	}

	var filename, src string
	// no file given (assume stdin), e.g. go-doctor refactor params...
	if r != nil {
		if stat, err := os.Stdin.Stat(); err != nil {
			printError(err)
		} else if stat.Size() < 1 {
			printError(fmt.Errorf("no filename given and no input given on stdin, see -h"))
		}
		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			printError(err)
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

	// do the refactoring
	results, err := refactor(filename, src, args, r)
	if err != nil {
		printError(err)
	}

	log := results.Log
	edits := results.Edits

	// TODO: do something with result.FSChanges -- e.g., Rename Package
	// will rename a directory

	// map[filename]output
	// where: output == diff || updated file
	changes := make(map[string][]byte)

	if log.ContainsErrors() {
		printResults(r.Description().Name, log, changes)
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
				printError(err)
			}

			var b bytes.Buffer
			p.Write(f, f, &b)
			changes[f] = b.Bytes()
		}
		printResults(r.Description().Name, log, changes)
	}

	// write all edits out to new file contents in []byte; something to work with
	for file, _ := range edits {
		if src != "" {
			str, err := doctor.ApplyToString(edits[file], src)
			if err != nil {
				printError(err)
			}
			changes[file] = []byte(str)
		} else {
			changes[file], err = doctor.ApplyToFile(edits[file], file)
			if err != nil {
				printError(err)
			}
		}
	}

	// write changes to their file and exit
	if *writeFlag {
		if *diffFlag || *formatFlag != "plain" {
			printError(fmt.Errorf("cannot write files with json or diff flags, try again"))
		}
		for file, change := range changes {
			if err := ioutil.WriteFile(file, change, 0); err != nil {
				printError(err)
			}
		}
		return
	}

	// so you came all this way for some output
	printResults(r.Description().Name, log, changes)
}

type Response struct {
	Reply string
	JSON  fields
	Plain []string
}

type fields map[string]interface{} // this got real old

func (r Response) String() string {
	var buf bytes.Buffer
	switch *formatFlag {
	case "plain":
		for i, p := range r.Plain {
			buf.WriteString(p)
			if i != len(r.Plain)-1 {
				buf.WriteString("\n")
			}
		}
	case "json":
		r.JSON["reply"] = r.Reply
		b, _ := json.MarshalIndent(r.JSON, "", "\t")
		buf.Write(b)
	default:
		return "invalid -format flag"
	}
	return buf.String()
}

func printError(err error) {
	r := Response{
		Reply: "Error",
		JSON:  fields{"message": err.Error()},
		Plain: []string{err.Error()},
	}
	fmt.Fprintf(os.Stderr, "%s\n", r)
	os.Exit(2)
}

func printResults(refactoring string, l *doctor.Log, changes map[string][]byte) {
	c := make(map[string]string)
	var contents []string
	var exitCode int

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

	// log to stderr if not json
	if *formatFlag == "plain" {
		for _, e := range l.Entries {
			exitCode = 1
			fmt.Fprintln(os.Stderr, e.String())
		}
	}

	fmt.Printf("%s\n", repl)
	os.Exit(exitCode)
}

func printRefactoringParams(r doctor.Refactoring) {
	params := []string{}
	for _, param := range r.Description().Params {
		params = append(params, param.Label)
	}
	resp := Response{"OK",
		fields{"params": params},
		params,
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

// TODO (jeff) figure out how to deal with scope/mainFile/2nd arg to SetSelection
// Anyway I think we need to know which file the main function is in,
// so I made that the second arg to SetSelection -- confirm with Alan
//
// This will do all of the configuration and execution for
// a refactoring r, returning the results.
func refactor(file string, src string, args []string, r doctor.Refactoring) (*doctor.Result, error) {
	if r == nil {
		return nil, fmt.Errorf("no refactoring given or in wrong place, see -h")
	}

	ts, err := parsePositionToTextSelection(*posFlag)
	if err != nil {
		return nil, err
	}

	var scope []string
	if *scopeFlag == "" {
		scope = append(scope, ts.Filename)
	} else {
		scope = parseScopes(*scopeFlag)
	}

	ts.Filename, err = filepath.Abs(file)
	if err != nil {
		return nil, err
	}

	var fs doctor.FileSystem
	if src != "" {
		// FIXME(reed): Need a filename for what's being passed on standard input -- must exist on the file system already -- then pass in absolute path to file in editor rather than "main.go"
		// FIXME(reed): Make sure the resulting edit set only changes the one file passed on stdin.  If it changes any others, bail with an error message
		fs, err = doctor.NewSingleEditedFileSystem("main.go", src)
		if err != nil {
			return nil, err
		}
	} else {
		fs = &doctor.LocalFileSystem{}
	}

	var argArray []interface{}
	for _, arg := range args { // cast to interface{}
		argArray = append(argArray, arg)
	}

	config := &doctor.Config{
		FileSystem: fs,
		Scope:      scope,
		Selection:  ts,
		Args:       argArray,
	}
	return r.Run(config), nil
}
