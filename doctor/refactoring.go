// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Refactoring interface, the RefactoringBase struct, and
// several methods common to refactorings based on RefactoringBase, including
// SetSelection, GetLog, and GetResult.

// Contributors: Jeff Overbey

package doctor

import (
	"code.google.com/p/go.tools/go/types"
	"code.google.com/p/go.tools/importer"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
)

// The Refactoring interface provides the methods common to all refactorings.
//
// The protocol for invoking a refactoring is:
//
//     1. Invoke SetSelection to initialize the refactoring and specify what
//        file is to be refactored.
//     2. Invoke any custom configuration methods (or Configure) to specify
//        any arguments.  For example, for the Rename refactoring, you must
//        specify a new name for the entity being renamed.
//     3. Invoke Run.
//     4. Invoke GetResult to get the resulting Log and EditSet.
//
// Name returns a human-readable name for the refactoring, properly capitalized
// (e.g., "Rename" or "Extract Function").  Every refactoring should have a
// unique name.
//
// Refactorings are typically invoked from a text editor.  The SetSelection
// method initializes the refactoring, clears the log (see GetLog/GetResult),
// and provides the refactoring with the file that was open in the text editor
// and the selected region/caret position.  The method returns true if the
// refactoring can be invoked on the given selection.  If the method returns
// false, more information may be obtained by invoking the GetLog method.
//
// The Configure method is used by the testing infrastructure to pass
// configuration information to the refactoring.  Test files are annotated
// with markers of the form
//     //<<<<<name,startline,startcol,endline,endcol,arg1,arg2,...,argn,pass
// which indicate what refactoring(s) to run.  The arguments arg1,arg2,...,argn
// (if present) are passed as the args of the Configure method.  This method
// returns false if configuration fails, i.e., if the wrong number of
// arguments are passed or the arguments are invalid.  If the method fails,
// more information may be obtained by invoking the GetLog method.
//
// The Run method runs the refactoring.
//
// Informational message, errors, and warnings are logged so that they can be
// displayed to the user.  This log can be obtained by invoking the GetLog
// method, or it can be obtained along with the resulting EditSet by invoking
// the GetResult method.
//
// If the log contains errors (log.ContainsErrors()), the resulting EditSet
// may be empty, since it may not be possible to perform the refactoring.
type Refactoring interface {
	Name() string
	SetSelection(selection TextSelection, mainFile string) bool
	Configure(args []string) bool
	Run()
	GetParams() []string
	GetLog() *Log
	GetResult() (*Log, EditSet)
}

type RefactoringBase struct {
	importer       *importer.Importer
	pkgInfo        *importer.PackageInfo
	file           *ast.File
	filename       string
	selectionStart token.Pos
	selectionEnd   token.Pos
	selectedNode   ast.Node
	log            *Log
	editSet        EditSet
}

// Configures a refactoring by indicating the filename in which text is
// selected and the beginning and end of the selected region.  Internally,
// this configures all of the fields in the RefactoringBase struct.  If
// nonempty, mainFile denotes the file containing the program entrypoint
// (main function), which may be different from the file containing the
// text selection.
func (r *RefactoringBase) SetSelection(selection TextSelection, mainFile string) bool {
	r.log = NewLog()

	r.filename = selection.filename
	//filename, err := filepath.Abs(selection.filename)
	//if err != nil {
	//	r.log.Log(FATAL_ERROR, err.Error())
	//	return false
	//}
	//r.filename = filename

	//	cwd, err := os.Getwd()
	//	if err != nil {
	//		r.log.Log(FATAL_ERROR, err.Error())
	//		return false
	//	}

	buildContext := build.Default
	if os.Getenv("GOPATH") != "" {
		// The test runner may change the GOPATH environment variable
		// since the program was started, so set it here explicitly
		buildContext.GOPATH = os.Getenv("GOPATH")
	}

	//r.importer = importer.New(new(importer.Config))
	//r.importer = importer.New(&importer.Config{Build: &build.Default})

	// r.importer = importer.New(&importer.Config{Build: &buildContext})

	var impcfg importer.Config
	impcfg.Build = &buildContext
	impcfg.TypeChecker.Error = func(err error) {
		// FIXME: Needs to be thread-safe
		// As of today, you can access the components of the error (token.Pos, string) as:
		// err.(types.Error).Pos etc.
		var message string = err.Error()
		var pos token.Pos = err.(types.Error).Pos
		var offset int = err.(types.Error).Fset.Position(pos).Offset
		var filename string = err.(types.Error).Fset.File(pos).Name()
		var length int = 0
		r.log.LogInitial(ERROR, message, filename, offset, length)
	}
	r.importer = importer.New(&impcfg)

	var pkgInfo []*importer.PackageInfo
	var err error
	if mainFile != "" {
		pkgInfo, _, err = r.importer.LoadInitialPackages([]string{r.filename, mainFile})
	} else {
		pkgInfo, _, err = r.importer.LoadInitialPackages([]string{r.filename})
	}
	if err != nil {
		r.log.Log(FATAL_ERROR, err.Error())
		return false
	} else if len(pkgInfo) < 1 {
		r.log.Log(FATAL_ERROR, "Analysis error: unable to import package(s)")
		return false
	}

	r.pkgInfo = pkgInfo[0]
	// Unnecessary since we hooked into the importer's error reporter
	//	if r.pkgInfo.Err != nil {
	//		r.log.Log(FATAL_ERROR, r.pkgInfo.Err.Error())
	//		return false
	//	}

	if len(r.pkgInfo.Files) < 1 {
		r.log.Log(FATAL_ERROR, "Package contains no files")
		return false
	}

	r.file = nil
	for _, file := range r.pkgInfo.Files {
		if r.importer.Fset.Position(file.Pos()).Filename == selection.filename {
			r.file = file
			break;
		}
	}
	if r.file == nil {
		r.log.Log(FATAL_ERROR, "Unable to parse " + selection.filename)
		return false
	}

	r.selectionStart = r.lineColToPos(selection.startLine, selection.startCol)
	r.selectionEnd = r.lineColToPos(selection.endLine, selection.endCol)

	nodes, _ := importer.PathEnclosingInterval(r.file,
		r.selectionStart, r.selectionEnd)
	r.selectedNode = nodes[0]

	r.editSet = NewEditSet()
	return true
}

//// Parses the given file, logging errors to the given log, and returning both
//// a FileSet and a File
//func (r *RefactoringBase) parse(filename string) *ast.File {
//	//f, err := parser.ParseFile(r.importer.Fset, filename, nil, parser.ParseComments)
//	fs, err := importer.ParseFiles(r.importer.Fset, ".", filename)
//	if err != nil {
//		r.log.Log(FATAL_ERROR, "Error parsing "+filename+": "+
//			err.Error())
//		return nil
//	}
//	if len(fs) != 1 {
//		r.log.Log(FATAL_ERROR, "Unable to parse " + filename)
//		return nil
//	}
//	return fs[0]
//}

// Converts a line/column position (where the first character in a file is at
// line 1, column 1) into a token.Pos
func (r *RefactoringBase) lineColToPos(line int, column int) token.Pos {
	file := r.importer.Fset.File(r.file.Pos())
	if file == nil {
		panic("file is nil")
	}
	lastLine := -1
	thisColumn := 1
	for i := 0; i < file.Size(); i++ {
		pos := file.Pos(i)
		thisLine := file.Line(pos)
		if thisLine != lastLine {
			thisColumn = 1
		} else {
			thisColumn++
		}
		if thisLine == line && thisColumn == column {
			return pos
		}
		lastLine = thisLine
	}
	return r.file.Pos()
}

func (r *RefactoringBase) checkForErrors() {
	contents, err := ioutil.ReadFile(r.filename)
	if err != nil {
		r.log.Log(ERROR, "Unable to read source file: "+err.Error())
		return
	}
	sourceFromFile := string(contents)

	string, err := r.editSet.ApplyToString(r.filename, sourceFromFile)
	if err != nil {
		r.log.Log(ERROR, "Transformation produced invalid EditSet: "+
			err.Error())
		return
	}

	f, err := parser.ParseFile(r.importer.Fset, "", string, parser.ParseComments)
	if err != nil {
		fmt.Println("vvvvv")
		fmt.Println(string)
		fmt.Println("^^^^^")
		r.log.Log(ERROR, "Transformation will introduce "+
			"syntax errors: "+err.Error())
		return
	}

	// TODO: This may be wrong if several files are changed...?
	r.pkgInfo = r.importer.CreatePackage(r.file.Name.Name, f)
	if r.pkgInfo.Err != nil {
		r.log.Log(ERROR, "Transformation will introduce semantic "+
			"errors: "+r.pkgInfo.Err.Error())
		return
	}
}

func (r *RefactoringBase) GetLog() *Log {
	return r.log
}

func (r *RefactoringBase) GetResult() (*Log, EditSet) {
	return r.log, r.editSet
}

func (r *RefactoringBase) forEachFile(f func(file *token.File, ast *ast.File)) {
	r.importer.Fset.Iterate(func(tfile *token.File) bool {
		r.findFile(tfile, f)
		return true
	})
}

func (r *RefactoringBase) findFile(tfile *token.File, f func(file *token.File, ast *ast.File)) {
	filename := tfile.Name()
	for _, pkgInfo := range r.importer.AllPackages() {
		for _, file := range pkgInfo.Files {
			pkgInfoFilename := r.importer.Fset.Position(file.Pos()).Filename
			if pkgInfoFilename == filename {
				f(tfile, file)
				return
			}
		}
	}
	log.Fatalf("Unable to find file %s in importer.AllPackages()", filename)
}
