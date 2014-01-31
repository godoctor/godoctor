// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Refactoring interface, the RefactoringBase struct, and
// several methods common to refactorings based on RefactoringBase, including
// SetSelection, GetLog, and GetResult.

// Package doctor provides infrastructure for building refactorings and similar
// source code-level program transformations for Go programs.
package doctor

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.google.com/p/go.tools/astutil"
	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
)

// All available refactorings, keyed by a unique, one-short, all-lowercase name
var refactorings map[string]Refactoring

func init() {
	refactorings = map[string]Refactoring{
		"rename":        new(RenameRefactoring),
		"reverseassign": new(ReverseAssignRefactoring),
		"shortassign":   new(ShortAssignRefactoring),
		"debug":         new(debugRefactoring),
		//"extract":	 new(ExtractRefactoring),
		"null": new(NullRefactoring),
	}
}

// AllRefactorings returns all of the transformations that can be performed.
// The keys of the returned map are short, single-word, all-lowercase names
// (rename, fiximports, etc.); the values implement the Refactoring interface.
func AllRefactorings() map[string]Refactoring {
	return refactorings
}

// GetRefactoring returns a Refactoring keyed by the given short name.  The
// short name must be one of the keys in the map returned by AllRefactorings.
func GetRefactoring(shortName string) Refactoring {
	return refactorings[shortName]
}

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
//     4. Invoke GetResult to get the resulting Log and the EditSet for each
//        file.
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
// method, or it can be obtained along with the resulting EditSets by invoking
// the GetResult method.
//
// If the log contains errors (log.ContainsErrors()), the resulting map of
// EditSets may be empty or incomplete, since it may not be possible to perform
// the refactoring.
type Refactoring interface {
	Name() string
	SetSelection(selection TextSelection, scope []string) bool
	Configure(args []string) bool
	Run()
	GetParams() []string
	GetLog() *Log
	GetResult() (*Log, map[string]EditSet)
}

type RefactoringBase struct {
	program        *loader.Program
	file           *ast.File
	selectionStart token.Pos
	selectionEnd   token.Pos
	selectedNode   ast.Node
	log            *Log
	editSet        map[string]EditSet
}

// Configures a refactoring by indicating the filename in which text is
// selected and the beginning and end of the selected region.  Internally, this
// configures all of the fields in the RefactoringBase struct.  If nonempty,
// scope denotes a scope (passed to the go.tools loader): typically a package
// name or a file containing the program entrypoint (main function), which may
// be different from the file containing the text selection.
func (r *RefactoringBase) SetSelection(selection TextSelection, scope []string) bool {
	r.log = NewLog()

	buildContext := build.Default
	if os.Getenv("GOPATH") != "" {
		// The test runner may change the GOPATH environment variable
		// since the program was started, so set it here explicitly
		// (not necessary when run as a CLI tool, but necessary when
		// run from refactoring_test.go)
		buildContext.GOPATH = os.Getenv("GOPATH")
	}

	var config loader.Config
	config.Build = &buildContext
	config.SourceImports = true
	config.TypeChecker.Error = func(err error) {
		// FIXME: Needs to be thread-safe
		var message string = err.Error()
		var pos token.Pos = err.(types.Error).Pos
		var offset int = err.(types.Error).Fset.Position(pos).Offset
		var filename string = err.(types.Error).Fset.File(pos).Name()
		var length int = 0
		r.log.LogInitial(ERROR, message, filename, offset, length)
	}
	// FIXME: Jeff: handle error

	if scope != nil {
		config.FromArgs(scope)
	} else {
		config.FromArgs([]string{selection.Filename})
	}

	var err error
	r.program, err = config.Load()
	if err != nil {
		r.log.Log(FATAL_ERROR, err.Error())
		return false
	} else if r.program == nil {
		r.log.Log(FATAL_ERROR, "Internal Error: Loader failed")
		return false
	}

	var pkgInfo *loader.PackageInfo
	pkgInfo, r.file = r.fileNamed(selection.Filename)
	if pkgInfo == nil || r.file == nil {
		r.log.Log(FATAL_ERROR,
			fmt.Sprintf("The selected file, %s, was not "+
				"found in the provided scope: %s",
				selection.Filename,
				scope))
		return false
	}

	r.selectionStart = r.lineColToPos(r.file, selection.StartLine, selection.StartCol)
	r.selectionEnd = r.lineColToPos(r.file, selection.EndLine, selection.EndCol)

	nodes, _ := astutil.PathEnclosingInterval(r.file,
		r.selectionStart, r.selectionEnd)
	r.selectedNode = nodes[0]

	r.editSet = map[string]EditSet{r.filename(r.file): NewEditSet()}

	return true
}

// lineColToPos converts a line/column position (where the first character in a
// file is at // line 1, column 1) into a token.Pos
func (r *RefactoringBase) lineColToPos(file *ast.File, line int, column int) token.Pos {
	if file == nil {
		panic("file is nil")
	}
	lastLine := -1
	thisColumn := 1
	tfile := r.program.Fset.File(file.Package)
	for i, size := 0, tfile.Size(); i < size; i++ {
		pos := tfile.Pos(i)
		thisLine := tfile.Line(pos)
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
	return file.Pos()
}

func (r *RefactoringBase) checkForErrors() {
	contents, err := ioutil.ReadFile(r.filename(r.file))
	if err != nil {
		r.log.Log(ERROR, "Unable to read source file: "+err.Error())
		return
	}
	sourceFromFile := string(contents)

	string, err := ApplyToString(r.editSet[r.filename(r.file)], sourceFromFile)
	if err != nil {
		r.log.Log(ERROR, "Transformation produced invalid EditSet: "+
			err.Error())
		return
	}

	_, err = parser.ParseFile(r.program.Fset, "", string, parser.ParseComments)
	if err != nil {
		fmt.Println("vvvvv")
		fmt.Println(string)
		fmt.Println("^^^^^")
		r.log.Log(ERROR, "Transformation will introduce "+
			"syntax errors: "+err.Error())
		return
	}

	/*
		// TODO: This may be wrong if several files are changed...?
		r.pkgInfo = r.program.CreatePackage(r.file.Name.Name, f)
		if r.pkgInfo.Err != nil {
			r.log.Log(ERROR, "Transformation will introduce semantic "+
				"errors: "+r.pkgInfo.Err.Error())
			return
		}
	*/
}

/*
//find occurrences of [top level] identifier across package
//TODO filter file? /dumbfounded
func (r *RefactoringBase) findAnyOccurrences(name string) bool {
	result := false
	//ast.Inspect(r.file, func(n ast.Node) bool {
	//switch thisIdent := n.(type) {
	//case *ast.Ident:
	//if r.pkgInfo.ObjectOf(thisIdent) == decl {
	//offset := r.program.Fset.Position(thisIdent.NamePos).Offset
	//length := utf8.RuneCountInString(thisIdent.Name)
	//result = append(result, OffsetLength{offset, length})
	//}
	//}

	//return true
	//})
	return result
}
*/

func (r *RefactoringBase) GetLog() *Log {
	return r.log
}

func (r *RefactoringBase) GetResult() (*Log, map[string]EditSet) {
	return r.log, r.editSet
}

/* -=-=- Utility Methods -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

func (r *RefactoringBase) pkgInfo(file *ast.File) *loader.PackageInfo {
	for _, pkgInfo := range r.program.AllPackages {
		for _, thisFile := range pkgInfo.Files {
			if thisFile == file {
				return pkgInfo
			}
		}
	}
	return nil
}

func (r *RefactoringBase) filename(file *ast.File) string {
	return r.program.Fset.Position(file.Package).Filename
}

func (r *RefactoringBase) fileContaining(node ast.Node) *ast.File {
	tfile := r.program.Fset.File(node.Pos())
	for _, pkgInfo := range r.program.AllPackages {
		for _, thisFile := range pkgInfo.Files {
			thisTFile := r.program.Fset.File(thisFile.Package)
			if thisTFile == tfile {
				return thisFile
			}
		}
	}
	panic("No ast.File for node")
}

func (r *RefactoringBase) fileNamed(filename string) (*loader.PackageInfo, *ast.File) {
	absFilename, _ := filepath.Abs(filename)
	for _, pkgInfo := range r.program.AllPackages {
		for _, f := range pkgInfo.Files {
			thisFile := r.program.Fset.Position(f.Pos()).Filename
			if thisFile == filename || thisFile == absFilename {
				return pkgInfo, f
			}
		}
	}
	return nil, nil
}

func (r *RefactoringBase) forEachFile(f func(ast *ast.File)) {
	for _, pkgInfo := range r.program.AllPackages {
		for _, ast := range pkgInfo.Files {
			f(ast)
		}
	}
}

func (r *RefactoringBase) forEachInitialFile(f func(ast *ast.File)) {
	for _, pkgInfo := range r.program.InitialPackages() {
		for _, ast := range pkgInfo.Files {
			f(ast)
		}
	}
}

func (r *RefactoringBase) readFromFile(offset, len int) string {
	buf := make([]byte, len)
	file, err := os.Open(r.filename(r.file))
	if err != nil {
		r.log.Log(FATAL_ERROR, fmt.Sprintf("Error on file Open %s", err))
	}
	defer file.Close()
	_, err = file.ReadAt(buf, int64(offset))
	if err != nil {
		r.log.Log(FATAL_ERROR, fmt.Sprintf("Error on file Open %s", err))
	}
	return string(buf)
}

func (r *RefactoringBase) offsetLength(node ast.Node) (int, int) {
	return r.program.Fset.Position(node.Pos()).Offset, (r.program.Fset.Position(node.End()).Offset - r.program.Fset.Position(node.Pos()).Offset)
}
