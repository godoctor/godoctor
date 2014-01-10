// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Refactoring interface, the RefactoringBase struct, and
// several methods common to refactorings based on RefactoringBase, including
// SetSelection, GetLog, and GetResult.

// Contributors: Jeff Overbey

// Package doctor provides infrastructure for building refactorings and similar
// source code-level program transformations for Go programs.
package doctor

import (
	"code.google.com/p/go.tools/astutil"
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
	"path/filepath"
	"unicode/utf8"
)

// All available refactorings, keyed by a unique, one-short, all-lowercase name
var refactorings map[string]Refactoring

func init() {
	refactorings = map[string]Refactoring{
		"null":        new(NullRefactoring),
		"rename":      new(RenameRefactoring),
		"shortassign": new(ShortAssignRefactoring),
		"fiximports":  new(FixImportsTransformation),
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
	SetSelection(selection TextSelection, mainFile string) bool
	Configure(args []string) bool
	Run()
	GetParams() []string
	GetLog() *Log
	GetResult() (*Log, map[string]EditSet)
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
	editSet        map[string]EditSet
}

// Configures a refactoring by indicating the filename in which text is
// selected and the beginning and end of the selected region.  Internally,
// this configures all of the fields in the RefactoringBase struct.  If
// nonempty, mainFile denotes the file containing the program entrypoint
// (main function), which may be different from the file containing the
// text selection.
func (r *RefactoringBase) SetSelection(selection TextSelection, mainFile string) bool {
	r.log = NewLog()

	r.filename = selection.Filename
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
		//fmt.Println("GOPATH is ",GOPATH)
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

	pkgInfo, err := r.loadPackages(mainFile)
	if len(pkgInfo) < 1 {
		r.log.Log(FATAL_ERROR, "Analysis error: unable to import package(s)")
		if err != nil {
			r.log.Log(FATAL_ERROR, err.Error())
		}
		return false
	} else if r.pkgInfo == nil || r.file == nil {
		r.log.Log(FATAL_ERROR, "Unable to parse "+selection.Filename)
		if err != nil {
			r.log.Log(FATAL_ERROR, err.Error())
		}
		return false
	}

	r.selectionStart = r.lineColToPos(selection.StartLine, selection.StartCol)
	r.selectionEnd = r.lineColToPos(selection.EndLine, selection.EndCol)

	nodes, _ := astutil.PathEnclosingInterval(r.file,
		r.selectionStart, r.selectionEnd)
	r.selectedNode = nodes[0]

	r.editSet = map[string]EditSet{selection.Filename: NewEditSet()}
	return true
}

// Finds all of the references in an AST to a single declaration
//@all = all Packages or just this Package
func (r *RefactoringBase) findOccurrences(all bool, ident *ast.Ident) map[string][]OffsetLength {

	//filenames to offsets
	result := make(map[string][]OffsetLength)

	decl := r.pkgInfo.ObjectOf(ident)
	if decl == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
		return nil
	}

	var pkgs []*importer.PackageInfo
	if all {
		for _, pkgInfo := range r.importer.AllPackages() {
			pkgs = append(pkgs, pkgInfo)
		}
	} else {
		pkgs = append(pkgs, r.pkgInfo)
	}

	for _, pkgInfo := range pkgs {
		for _, f := range pkgInfo.Files {
			//inspect each file in package for identifier
			ast.Inspect(f, func(n ast.Node) bool {
				switch thisIdent := n.(type) {
				case *ast.Ident:
					if r.pkgInfo.ObjectOf(thisIdent) == decl {
						offset := r.importer.Fset.Position(thisIdent.NamePos).Offset
						length := utf8.RuneCountInString(thisIdent.Name)
						filename := r.importer.Fset.Position(f.Pos()).Filename
						result[filename] = append(result[filename], OffsetLength{offset, length})
					}
				}
				return true
			})
		}
	}
	return result
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

func (r *RefactoringBase) loadPackages(mainFile string) (
	pkgInfo []*importer.PackageInfo, err error) {
	if mainFile != "" {
		pkgInfo, _, err = r.importer.LoadInitialPackages(
			[]string{mainFile})
	}

	var wasAlreadyLoaded bool
	r.pkgInfo, r.file, wasAlreadyLoaded, err = r.ensureIsLoaded(r.filename)
	if !wasAlreadyLoaded {
		pkgInfo = append(pkgInfo, r.pkgInfo)
	}

	return pkgInfo, err
}

func (r *RefactoringBase) ensureIsLoaded(filename string) (pkgInfo *importer.PackageInfo, file *ast.File, wasAlreadyLoaded bool, err error) {

	// If the importer already loaded this file, do not load it again
	pkgInfo, file = r.getFileFromImporter(filename)
	if pkgInfo != nil && file != nil {
		return pkgInfo, file, true, nil
	}

	// Determine this file's package and load the package if possible
	pkg, err := r.determinePackage(r.filename)
	if err != nil || pkg == "" || pkg == "main" {
		_, err = r.importer.ImportPackage(r.filename)
	} else {
		_, err = r.importer.ImportPackage(pkg)
	}

	pkgInfo, file = r.getFileFromImporter(filename)
	return pkgInfo, file, false, err
}

func (r *RefactoringBase) getFileFromImporter(filename string) (*importer.PackageInfo, *ast.File) {
	absFilename, _ := filepath.Abs(filename)
	for _, pkgInfo := range r.importer.AllPackages() {
		for _, f := range pkgInfo.Files {
			thisFile := r.importer.Fset.Position(f.Pos()).Filename
			if thisFile == filename || thisFile == absFilename {
				return pkgInfo, f
			}
		}
	}
	return nil, nil
}

func (r *RefactoringBase) determinePackage(filename string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.PackageClauseOnly)
	if err != nil {
		return "", err
	}
	if f.Name == nil {
		return "", nil
	}
	return f.Name.Name, nil
}

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

	string, err := ApplyToString(r.editSet[r.filename], sourceFromFile)
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

//find occurrences of [top level] identifier across package
//TODO filter file? /dumbfounded
func (r *RefactoringBase) findAnyOccurrences(name string) bool {
	result := false
	//ast.Inspect(r.file, func(n ast.Node) bool {
	//switch thisIdent := n.(type) {
	//case *ast.Ident:
	//if r.pkgInfo.ObjectOf(thisIdent) == decl {
	//offset := r.importer.Fset.Position(thisIdent.NamePos).Offset
	//length := utf8.RuneCountInString(thisIdent.Name)
	//result = append(result, OffsetLength{offset, length})
	//}
	//}

	//return true
	//})
	return result
}

func (r *RefactoringBase) GetLog() *Log {
	return r.log
}

func (r *RefactoringBase) GetResult() (*Log, map[string]EditSet) {
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

func closure(allInterfaces []*types.Interface, allConcreteTypes []types.Type) map[types.Type][]types.Type {
	graph := digraphClosure(implementsGraph(allInterfaces, allConcreteTypes))

	result := make(map[types.Type][]types.Type, len(allInterfaces)+len(allConcreteTypes))
	for u, adj := range graph {
		typ := mapType(u, allInterfaces, allConcreteTypes)
		typesAffected := make([]types.Type, 0, len(adj))
		for _, v := range adj {
			typesAffected = append(typesAffected, mapType(v, allInterfaces, allConcreteTypes))
		}
		result[typ] = typesAffected
	}
	return result
}

func implementsGraph(allInterfaces []*types.Interface, allConcreteTypes []types.Type) [][]int {
	adj := make([][]int, len(allInterfaces)+len(allConcreteTypes))
	for i, interf := range allInterfaces {
		for j, typ := range allConcreteTypes {
			if types.Implements(typ, interf, false) {
				adj[i] = append(adj[i], len(allInterfaces)+j)
				adj[len(allInterfaces)+j] = append(adj[len(allInterfaces)+j], i)
			}
		}
	}
	// TODO: Handle subtype relationships due to embedded structs
	return adj
}

func mapType(node int, allInterfaces []*types.Interface, allConcreteTypes []types.Type) types.Type {
	if node >= len(allInterfaces) {
		return allConcreteTypes[node-len(allInterfaces)]
	} else {
		return allInterfaces[node]
	}
}
