// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The cli package provides a command-line interface for the Go Doctor.
package cli

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"strings"

	"github.com/godoctor/godoctor/engine"
	"github.com/godoctor/godoctor/engine/protocol"
	"github.com/godoctor/godoctor/filesystem"
	"github.com/godoctor/godoctor/refactoring"
	"github.com/godoctor/godoctor/text"
)

const useHelp = "Run 'godoctor -help' for more information.\n"

func printHelp(flags *flag.FlagSet, stderr io.Writer) {
	fmt.Fprintln(stderr, `Go source code refactoring tool.
Usage: godoctor [<flag> ...] <refactoring> <args> ...

Each <flag> must be one of the following:`)
	flags.VisitAll(func(flag *flag.Flag) {
		fmt.Fprintf(stderr, "    -%-8s %s\n", flag.Name, flag.Usage)
	})
	fmt.Fprintln(stderr, `

The <refactoring> argument determines the refactoring to perform:`)
	for _, key := range engine.AllRefactoringNames() {
		r := engine.GetRefactoring(key)
		if !r.Description().Hidden {
			fmt.Fprintf(stderr, "    %-15s %s\n",
				key, r.Description().Synopsis)
		}
	}
	fmt.Fprintln(stderr, `
The <args> following the refactoring name vary depending on the refactoring.

To display usage information for a particular refactoring, such as rename, use:
    %% godoctor rename

For complete usage information, see the user manual:  FIXME: URL`)
}

// Run runs the Go Doctor command-line interface.  Typical usage is
//     os.Exit(cli.Run(os.Stdin, os.Stdout, os.Stderr, os.Args))
// All arguments must be non-nil, and args[0] is required.
func Run(stdin io.Reader, stdout io.Writer, stderr io.Writer, args []string) int {
	var flags *flag.FlagSet = flag.NewFlagSet("godoctor", flag.ContinueOnError)

	var fileFlag = flags.String("file", "",
		"Filename containing an element to refactor (default: standard input)")

	var posFlag = flags.String("pos", "1,1:1,1",
		"Position of a syntax element to refactor (default: entire file)")

	var scopeFlag = flags.String("scope", "",
		"Package name(s), or source file containing a program entrypoint")

	var completeFlag = flags.Bool("complete", false,
		"Output entire modified source files instead of displaying a diff")

	var writeFlag = flags.Bool("w", false,
		"Modify source files on disk (write) instead of displaying a diff")

	var verboseFlag = flags.Bool("v", false,
		"Verbose: list files if ≥ 1 file affected")

	var veryVerboseFlag = flags.Bool("vv", false,
		"Very verbose: list individual edits (implies -v)")

	var listFlag = flags.Bool("list", false,
		"List all refactorings and exit")

	var jsonFlag = flags.Bool("json", false,
		"Accept commands in OpenRefactory JSON protocol format")

	// Don't print full help unless -help was requested.
	// Just gently remind users that it's there.
	flags.Usage = func() { fmt.Fprint(stderr, useHelp) }
	flags.Init(args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args[1:]); err != nil {
		// (err has already been printed)
		if err == flag.ErrHelp {
			// Invoked as "godoctor [flags] -help"
			printHelp(flags, stderr)
			return 2
		}
		return 1
	}

	args = flags.Args()

	if *listFlag {
		if len(args) > 0 {
			fmt.Fprintln(stderr, "Error: The -list flag "+
				"cannot be used with any arguments")
			return 1
		}
		if *verboseFlag || *veryVerboseFlag || *writeFlag || *completeFlag || *jsonFlag {
			fmt.Fprintln(stderr, "Error: The -list flag "+
				"cannot be used with the -v, -vv, -w, "+
				"-complete, or -json flags")
			return 1
		}
		// Invoked as "godoctor [-v] [-file=""] [-pos=""] -list
		fmt.Fprintf(stderr, "%-15s\t%-47s\t%s\n",
			"Refactoring", "Description", "     Multifile?")
		fmt.Fprintf(stderr, "--------------------------------------------------------------------------------\n")
		for _, key := range engine.AllRefactoringNames() {
			r := engine.GetRefactoring(key)
			d := r.Description()
			if !r.Description().Hidden {
				fmt.Fprintf(stderr, "%-15s\t%-50s\t%v\n",
					key, d.Synopsis, d.Multifile)
			}
		}
		return 0
	}

	if *jsonFlag {
		if flags.NFlag() != 1 {
			fmt.Fprintln(stderr, "Error: The -json flag "+
				"cannot be used with any other flags")
			return 1
		}
		// Invoked as "godoctor -json [args]
		protocol.Run(args)
		return 0
	}

	if *writeFlag && *completeFlag {
		fmt.Fprintln(stderr, "Error: The -w and -complete flags "+
			"cannot both be present")
		return 1
	}

	if len(args) == 0 || args[0] == "" || args[0] == "help" {
		// Invoked as "godoctor [flags]" or "godoctor [flags] help"
		printHelp(flags, stderr)
		return 2
	}

	refacName := args[0]
	refac := engine.GetRefactoring(refacName)
	if refac == nil {
		fmt.Fprintf(stderr, "There is no refactoring named \"%s\"\n",
			refacName)
		return 1
	}

	args = args[1:]

	if flags.NFlag() == 0 && flags.NArg() == 1 {
		// Invoked as "godoctor refactoring"
		fmt.Fprintf(stderr, "Usage: %s %s\n",
			refacName, refac.Description().Usage)
		return 2
	}

	stdinPath := ""

	var fileName string
	var fileSystem filesystem.FileSystem
	if *fileFlag != "" && *fileFlag != "-" {
		fileName = *fileFlag
		fileSystem = &filesystem.LocalFileSystem{}
	} else {
		// Filename is - or no filename given; read from standard input
		var err error
		stdinPath, err = filesystem.FakeStdinPath()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fileName = stdinPath
		bytes, err := ioutil.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fileSystem, err = filesystem.NewSingleEditedFileSystem(
			stdinPath, string(bytes))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	selection, err := text.NewSelection(fileName, *posFlag)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %s.\n", err)
		return 1
	}

	var scope []string
	if *scopeFlag == "" {
		// If no scope provided, let refactoring.go guess the scope
		scope = nil
	} else if *scopeFlag == "-" && stdinPath != "" {
		// Use -scope=- to indicate "stdin file (not package) scope"
		scope = []string{stdinPath}
	} else {
		// Use -scope=a,b,c to specify multiple files/packages
		scope = strings.Split(*scopeFlag, ",")
	}

	verbosity := 0
	if *verboseFlag {
		verbosity = 1
	}
	if *veryVerboseFlag {
		verbosity = 2
	}

	result := refac.Run(&refactoring.Config{
		FileSystem: fileSystem,
		Scope:      scope,
		Selection:  selection,
		Args:       refactoring.InterpretArgs(args, refac),
		Verbosity:  verbosity})

	// Display log in GNU-style 'file:line.col-line.col: message' format
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	result.Log.Write(stderr, cwd)

	// If input was supplied on standard input, ensure that the refactoring
	// makes changes only to that code (and does not affect any other files)
	if stdinPath != "" {
		for f, _ := range result.Edits {
			if f != stdinPath {
				fmt.Fprintf(stderr, "Error: When source code is given on standard input, refactorings are prohibited from changing any other files.  This refactoring would require modifying %s.\n", f)
				return 1
			}
		}
	}

	if *writeFlag {
		err = writeToDisk(result, fileSystem)
	} else if *completeFlag {
		err = writeFileContents(stdout, result.Edits, fileSystem)
	} else {
		err = writeDiff(stdout, result.Edits, fileSystem)
	}
	if err != nil {
		fmt.Fprintf(stderr, "Error: %s.\n", err)
		return 1
	}

	if result.Log.ContainsErrors() {
		return 3
	} else {
		return 0
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
			stdinPath, _ := filesystem.FakeStdinPath()
			if f == stdinPath {
				inFile = os.Stdin.Name()
				outFile = os.Stdout.Name()
			} else {
				rel := relativePath(f)
				inFile = rel
				outFile = rel
			}
			fmt.Fprintf(out, "diff -u %s %s\n", inFile, outFile)
			p.Write(inFile, outFile, time.Time{}, time.Time{}, out)
		}
	}
	return nil
}

// relativePath returns a relative path to fname, or fname if a relative path
// cannot be computed due to an error
func relativePath(fname string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, fname); err == nil {
			return rel
		}
	}
	return fname
}

// writeFileContents outputs the complete contents of each file affected by
// this refactoring.
func writeFileContents(out io.Writer, edits map[string]*text.EditSet, fs filesystem.FileSystem) error {
	for filename, edits := range edits {
		data, err := filesystem.ApplyEdits(edits, fs, filename)
		if err != nil {
			return err
		}

		stdinPath, _ := filesystem.FakeStdinPath()
		if filename == stdinPath {
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
	return nil
}