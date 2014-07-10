// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The godoctor command refactors Go code.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"strings"

	"golang-refactoring.org/go-doctor/engine"
	"golang-refactoring.org/go-doctor/engine/protocol"
	"golang-refactoring.org/go-doctor/filesystem"
	"golang-refactoring.org/go-doctor/refactoring"
	"golang-refactoring.org/go-doctor/text"
)

var fileFlag = flag.String("file", "",
	"Filename containing an element to refactor (default: standard input)")

var posFlag = flag.String("pos", "1,1:1,1",
	"Position of a syntax element to refactor (default: entire file)")

var scopeFlag = flag.String("scope", "",
	"Package name(s), or source file containing a program entrypoint")

var completeFlag = flag.Bool("complete", false,
	"Output entire modified source files instead of displaying a diff")

var writeFlag = flag.Bool("w", false,
	"Modify source files on disk (write) instead of displaying a diff")

var verboseFlag = flag.Bool("v", false,
	"Log all edits made by the refactoring (verbose)")

var listFlag = flag.Bool("list", false,
	"List all refactoring names and exit")

var jsonFlag = flag.Bool("json", false,
	"Accept commands in OpenRefactory JSON protocol format")

const useHelp = "Run 'godoctor -help' for more information.\n"

func printHelp() {
	fmt.Fprintln(os.Stderr, `Go source code refactoring tool.
Usage: godoctor [<flag> ...] <refactoring> <args> ...

Each <flag> must be one of the following:`)
	flag.CommandLine.VisitAll(func(flag *flag.Flag) {
		fmt.Fprintf(os.Stderr, "    -%-8s %s\n", flag.Name, flag.Usage)
	})
	fmt.Fprintln(os.Stderr, `

The <refactoring> argument determines the refactoring to perform:`)
	for key, r := range engine.AllRefactorings() {
		//if r.Description().Quality == refactoring.Production {
		// FIXME: One-line description
		fmt.Fprintf(os.Stderr, "    %-15s %s\n", key, r.Description().Name)
		//}
	}
	fmt.Fprintln(os.Stderr, `
The <args> following the refactoring name vary depending on the refactoring.

To display usage information for a particular refactoring, such as rename, use:
    %% godoctor rename

For complete usage information, see the user manual:  FIXME: URL`)
}

func main() {
	// Don't print full help unless -help was requested.
	// Just gently remind users that it's there.
	flag.Usage = func() { fmt.Fprint(os.Stderr, useHelp) }
	flag.CommandLine.Init(os.Args[0], flag.ExitOnError)
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		// (err has already been printed)
		if err == flag.ErrHelp {
			// Invoked as "godoctor [flags] -help"
			printHelp()
		}
		os.Exit(2)
	}

	args := flag.Args()

	if *listFlag {
		if len(args) > 0 {
			fmt.Fprintln(os.Stderr, "Error: The -list flag "+
				"cannot be used with any arguments")
			os.Exit(1)
		}
		if *writeFlag || *completeFlag || *jsonFlag {
			fmt.Fprintln(os.Stderr, "Error: The -list flag "+
				"cannot be used with the -w, -complete, or "+
				"-json flags")
			os.Exit(1)
		}
		// Invoked as "godoctor [-v] [-file=""] [-pos=""] -list
		for key, _ := range engine.AllRefactorings() {
			//if r.Description().Quality == refactoring.Production {
			// FIXME: One-line description
			fmt.Fprintf(os.Stderr, "%s\n", key)
			//}
		}
		return
	}

	if len(args) == 0 || args[0] == "" || args[0] == "help" {
		// Invoked as "godoctor [flags]" or "godoctor [flags] help"
		printHelp()
		os.Exit(2)
	}

	if *jsonFlag {
		if flag.NFlag() != 1 {
			fmt.Fprintln(os.Stderr, "Error: The -json flag "+
				"cannot be used with any other flags")
			os.Exit(1)
		}
		// Invoked as "godoctor -json [args]
		protocol.Run(args)
		return
	}

	if *writeFlag && *completeFlag {
		fmt.Fprintln(os.Stderr, "Error: The -w and -complete flags "+
			"cannot both be present")
		os.Exit(1)
	}

	refac := engine.GetRefactoring(args[0])
	args = args[1:]
	if refac == nil {
		fmt.Fprintf(os.Stderr, "There is no refactoring named \"%s\"\n", args[0])
		os.Exit(1)
	}

	if flag.NFlag() == 0 && flag.NArg() == 1 {
		// Invoked as "godoctor refactoring"
		fmt.Fprintf(os.Stderr, "FIXME: refactoring usage\n")
		os.Exit(2)
	}

	stdin := ""

	var fileName string
	var fileSystem filesystem.FileSystem
	if *fileFlag != "" {
		fileName = *fileFlag
		fileSystem = &filesystem.LocalFileSystem{}
	} else {
		// No filename; read from standard input
		stdin, err := filesystem.FakeStdinPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fileName = stdin
		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fileSystem, err = filesystem.NewSingleEditedFileSystem(
			stdin, string(bytes))
		if err != nil {
			fmt.Fprintln(os.Stderr, "***")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	selection, err := text.NewSelection(fileName, *posFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s.\n", err)
		os.Exit(1)
	}

	var scope []string
	if *scopeFlag == "" {
		scope = nil
	} else {
		scope = strings.Split(*scopeFlag, ",")
	}

	result := refac.Run(&refactoring.Config{
		FileSystem: fileSystem,
		Scope:      scope,
		Selection:  selection,
		Args:       refactoring.InterpretArgs(args, refac),
		Verbose:    *verboseFlag})

	// Display log in GNU-style 'file:line.col-line.col: message' format
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	result.Log.Write(os.Stderr, cwd)

	// If input was supplied on standard input, ensure that the refactoring
	// makes changes only to that code (and does not affect any other files)
	if stdin != "" {
		for f, _ := range result.Edits {
			if f != stdin {
				fmt.Fprintf(os.Stderr, "Error: When source code is given on standard input, refactorings are prohibited from changing any other files.  This refactoring would require modifying %s.\n", f)
				os.Exit(1)
			}
		}
		if len(result.FSChanges) > 0 {
			fmt.Fprintf(os.Stderr, "Error: When source code is given on standard input, refactorings are prohibited from changing any other files.  This refactoring would require a change to the file system (%s).\n", result.FSChanges[0])
			os.Exit(1)
		}
	}

	if *writeFlag {
		err = writeToDisk(result, fileSystem)
	} else if *completeFlag {
		err = writeFileContents(os.Stdout, result.Edits, fileSystem)
	} else {
		err = writeDiff(os.Stdout, result.Edits, fileSystem)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s.\n", err)
		os.Exit(1)
	}

	if !*writeFlag && len(result.FSChanges) > 0 {
		fmt.Fprintln(os.Stderr, "After applying the patch, the following file system changes must be made:")
		for _, chg := range result.FSChanges {
			fmt.Fprintf(os.Stderr, "    %s\n", chg.String(cwd))
		}
	}

}

// writeDiff outputs a multi-file unified diff describing this refactoring's
// changes.  It can be applied using GNU patch.
func writeDiff(out io.Writer, edits map[string]*text.EditSet, fs filesystem.FileSystem) error {
	for f, e := range edits {
		p, err := filesystem.CreatePatch(e, fs, f)
		if err != nil {
			return err
		}

		if !p.IsEmpty() {
			inFile := f
			outFile := f
			stdin, _ := filesystem.FakeStdinPath()
			if f == stdin {
				inFile = os.Stdin.Name()
				outFile = os.Stdout.Name()
			}
			fmt.Fprintf(out, "diff -u %s %s\n", inFile, outFile)
			p.Write(inFile, outFile, time.Time{}, time.Time{}, out)
		}
	}
	return nil
}

// writeFileContents outputs the complete contents of each file affected by
// this refactoring.
func writeFileContents(out io.Writer, edits map[string]*text.EditSet, fs filesystem.FileSystem) error {
	for filename, edits := range edits {
		data, err := filesystem.ApplyEdits(edits, fs, filename)
		if err != nil {
			return err
		}

		stdin, _ := filesystem.FakeStdinPath()
		if filename == stdin {
			filename = os.Stdin.Name()
		}

		if _, err := fmt.Fprintf(out, "@@@@@ %s @@@@@ %d @@@@@\n",
			filename, len(data)); err != nil {
			return err
		}
		n, err := out.Write(data)
		if n < len(data) && err == nil {
			err = io.ErrShortWrite
		}
		if err != nil {
			return err
		}
		if len(data) > 0 && data[len(data)-1] != '\n' {
			fmt.Fprintln(out)
		}
	}
	return nil
}

// writeToDisk overwrites existing files with their refactored versions and
// applies any other changes to the file system that the refactoring requires
// (e.g., renaming directories).
func writeToDisk(result *refactoring.Result, fs filesystem.FileSystem) error {
	for filename, edits := range result.Edits {
		data, err := filesystem.ApplyEdits(edits, fs, filename)
		if err != nil {
			return err
		}

		f, err := fs.OverwriteFile(filename)
		if err != nil {
			return err
		}
		n, err := f.Write(data)
		if err == nil && n < len(data) {
			err = io.ErrShortWrite
		}
		if err1 := f.Close(); err == nil {
			err = err1
		}
		if err != nil {
			return err
		}
	}
	for _, change := range result.FSChanges {
		if err := change.ExecuteUsing(fs); err != nil {
			return err
		}
	}
	return nil
}
