// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Refactoring interface, the refactoringBase struct, and
// several methods common to refactorings based on refactoringBase, including
// a base implementation of the Run method.

// Package refactoring contains all of the refactorings supported by the Go
// Doctor, as well as types (such as refactoring.Log) used to interface with
// those refactorings.
package refactoring

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"code.google.com/p/go.tools/astutil"
	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
	"golang-refactoring.org/go-doctor/filesystem"
	"golang-refactoring.org/go-doctor/text"
)

// All available refactorings, keyed by a unique, one-short, all-lowercase name
var refactorings map[string]Refactoring

func init() {
	refactorings = map[string]Refactoring{
		"rename":        new(renameRefactoring),
		"reverseassign": new(reverseAssignRefactoring),
		"shortassign":   new(shortAssignRefactoring),
		"godoc":         new(addGodocRefactoring),
		"debug":         new(debugRefactoring),
		"null":          new(nullRefactoring),
	}
}

// The maximum number of errors from the go/loader that will be reported
const maxInitialErrors = 10

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

// Description of a parameter for a refactoring.
//
// Some refactorings require additional input from the user besides a text
// selection.  For example, in a Rename refactoring, the user may select an
// identifier to rename, but the refactoring tool must also elicit (1) a new
// name for the identifier and (2) whether or not occurrences of the name
// should be replaced in comments.  These two inputs are parameters to the
// refactoring.
type Parameter struct {
	// A brief label suitable for display next to an input field (e.g., a
	// text box or check box in a dialog box), e.g., "Name:" or "Replace
	// occurrences"
	Label string
	// A longer (typically one sentence) description of the input
	// requested, suitable for display in a tooltip/hover tip.
	Prompt string
	// The default value for this parameter.  The type of the parameter
	// (string or boolean) can be determined from the type of its default
	// value.
	DefaultValue interface{}
}

// IsBoolean returns true iff this Parameter must be either true or false.
func (p *Parameter) IsBoolean() bool {
	switch p.DefaultValue.(type) {
	case bool:
		return true
	default:
		return false
	}
}

// Quality determines whether a refactoring is exposed to end users
type Quality int

const (
	// Refactoring should not be exposed to end users
	Development Quality = iota
	// Refactoring has not been extensively tested on large codes but is
	// stable enough for early adopters to try
	Testing
	// Refactoring can be safely used in a production environment
	Production
)

// Description provides information about a refactoring suitable for display in
// a user interface.
type Description struct {
	// A human-readable name for this refactoring, properly capitalized
	// (e.g., "Rename" or "Extract Function") as it would appear in a user
	// interface.  Every refactoring should have a unique name.
	Name string
	// Additional input required for this refactoring.  See Parameter.
	Params []Parameter
	// Whether this refactoring is suitable for production use.
	Quality Quality
}

// A Config provides the initial configuration for a refactoring, including the
// file system and program on which it will operate, the initial text
// selection, and any refactoring-specific arguments.
//
// At a minimum, the FileSystem, Scope, and Selection arguments must be set.
type Config struct {
	// The file system on which the refactoring will operate.
	FileSystem filesystem.FileSystem
	// A set of initial packages to load.  This slice will be passed as-is
	// to the Config.FromArgs method of go.tools/go/loader.  Typically, the
	// scope will consist of a package name or a file containing the
	// program entrypoint (main function), which may be different from the
	// file containing the text selection.
	Scope []string
	// The range of text on which to invoke the refactoring.
	Selection *text.Selection
	// Refactoring-specific arguments.  To determine what arguments are
	// required for each refactoring, see Refactoring.Description().Params.
	// For example, for the Rename refactoring, you must specify a new name
	// for the entity being renamed.  If the refactoring does not require
	// any arguments, this may be nil.
	Args []interface{}
	// The GOPATH.  If this is set to the empty string, the GOPATH is
	// determined from the environment.
	GoPath string
}

// The Refactoring interface identifies methods common to all refactorings.
//
// The protocol for invoking a refactoring is:
//
//     1. If necessary, invoke the Description() method to obtain the name of
//        the refactoring and a list of arguments that must be provided to it.
//     2. Create a Config.  Refactorings are typically invoked from a text
//        editor; the Config provides the refactoring with the file that was
//        open in the text editor and the selected region/caret position.
//     3. Invoke Run, which returns a Result.
//     4. If Result.Log is not empty, display the log to the user.
//     5. If Result.Edits or Result.FSChanges are non-nil, they may be applied
//        to complete the transformation.
type Refactoring interface {
	Description() *Description
	Run(*Config) *Result
}

type Result struct {
	// A list of informational messages, errors, and warnings to display to
	// the user.  If the Log.ContainsErrors() is true, the Edits and
	// FSChanges may be empty or incomplete, since it may not be possible
	// to perform the refactoring.
	Log *Log
	// Maps filenames to the text edits that should be applied to those
	// files.
	Edits map[string]*text.EditSet
	// File system changes: files and directories to rename, create, or
	// delete after the Edits have been applied.  These changes should be
	// applied in order, since changes later in the list may depend on the
	// successful completion of changes earlier in the list (e.g., a path
	// to a file may be invalid after its containing directory is renamed).
	FSChanges []filesystem.Change
}

const cgoError1 = "could not import C (cannot"
const cgoError2 = "undeclared name: C"

type refactoringBase struct {
	program        *loader.Program
	file           *ast.File
	fileContents   []byte
	selectionStart token.Pos
	selectionEnd   token.Pos
	selectedNode   ast.Node
	Result
}

// Base implementation of a Run method.  Most refactorings should invoke this
// method before performing refactoring-specific work.  This method
// initializes the refactoring, clears the log, and
// configures all of the fields in the refactoringBase struct.
func (r *refactoringBase) Run(config *Config) *Result {
	r.Log = NewLog()

	if config.FileSystem == nil {
		r.Log.Error("INTERNAL ERROR: null Config.FileSystem")
		return &r.Result
	}

	buildContext := build.Default
	if os.Getenv("GOPATH") != "" {
		// The test runner may change the GOPATH environment variable
		// since the program was started, so set it here explicitly
		// (not necessary when run as a CLI tool, but necessary when
		// run from refactoring_test.go)
		buildContext.GOPATH = os.Getenv("GOPATH")
	}
	if config.GoPath != "" {
		buildContext.GOPATH = config.GoPath
	}
	buildContext.ReadDir = config.FileSystem.ReadDir
	buildContext.OpenFile = config.FileSystem.OpenFile

	var lconfig loader.Config
	lconfig.Build = &buildContext
	lconfig.ParserMode = parser.ParseComments | parser.DeclarationErrors
	lconfig.AllowErrors = true
	lconfig.SourceImports = true
	lconfig.TypeChecker.Error = func(err error) {
		message := err.Error()
		// FIXME: Needs to be thread-safe
		// TODO: This is temporary until go/loader handles cgo
		if !strings.Contains(message, cgoError1) &&
			!strings.HasSuffix(message, cgoError2) &&
			len(r.Log.Entries) < maxInitialErrors {
			r.Log.Error(message)
			r.Log.AssociatePos(err.(types.Error).Fset,
				err.(types.Error).Pos,
				err.(types.Error).Pos)
		}
	}

	if config.Scope == nil {
		config.Scope = r.guessScope(config)
		r.Log.Warnf("No scope provided; guessing %s",
			strings.Join(config.Scope, " "))
	} else {
		r.Log.Infof("Scope is %s", strings.Join(config.Scope, " "))
	}
	lconfig.FromArgs(config.Scope, true)

	var err error
	r.program, err = lconfig.Load()
	r.Log.MarkInitial()
	if err != nil {
		r.Log.Error(err)
		return &r.Result
	} else if r.program == nil {
		r.Log.Error("INTERNAL ERROR: Loader failed")
		return &r.Result
	}

	var pkgInfo *loader.PackageInfo
	pkgInfo, r.file = r.fileNamed(config.Selection.Filename)
	if pkgInfo == nil || r.file == nil {
		r.Log.Errorf("The selected file, %s, was not found in the "+
			"provided scope: %s",
			config.Selection.Filename,
			config.Scope)
		// This can happen on files containing +build
		return &r.Result
	}

	r.selectionStart = r.lineColToPos(r.file, config.Selection.StartLine, config.Selection.StartCol)
	r.selectionEnd = r.lineColToPos(r.file, config.Selection.EndLine, config.Selection.EndCol)

	nodes, _ := astutil.PathEnclosingInterval(r.file,
		r.selectionStart, r.selectionEnd)
	r.selectedNode = nodes[0]

	reader, err := config.FileSystem.OpenFile(r.filename(r.file))
	if err != nil {
		r.Log.Errorf("Unable to open %s", r.filename(r.file))
		return &r.Result
	}
	r.fileContents, err = ioutil.ReadAll(reader)
	if err != nil {
		r.Log.Errorf("Unable to read %s", r.filename(r.file))
		return &r.Result
	}

	r.Edits = map[string]*text.EditSet{r.filename(r.file): text.NewEditSet()}
	r.FSChanges = []filesystem.Change{}

	return &r.Result
}

// guessScope makes a reasonable guess at the refactoring scope if the user
// does not provide an explicit scope.  It guesses as follows:
//     1. If filename is not in $GOPATH/src, filename is used as the scope.
//     2. If filename is in $GOPATH/src, a package name is guessed by stripping
//        $GOPATH/src/ from the filename, and that package is used as the scope.
func (r *refactoringBase) guessScope(config *Config) []string {
	fnameScope := []string{config.Selection.Filename}

	absFilename, err := filepath.Abs(config.Selection.Filename)
	if err != nil {
		r.Log.Error(err.Error())
		return fnameScope
	}

	gopath := config.GoPath
	if gopath == "" {
		gopath = os.Getenv("GOPATH")
	}
	if gopath == "" {
		r.Log.Warn("GOPATH not set")
		return fnameScope
	}
	gopath, err = filepath.Abs(gopath)
	if err != nil {
		r.Log.Error(err)
		return fnameScope
	}

	gopathSrc := filepath.Join(gopath, "src")

	relFilename, err := filepath.Rel(gopathSrc, absFilename)
	if err != nil {
		r.Log.Error(err)
		return fnameScope
	}

	if strings.HasPrefix(relFilename, "..") {
		return fnameScope
	}

	return []string{filepath.ToSlash(filepath.Dir(relFilename))}
}

// validateArgs determines whether the arguments supplied in the given Config
// match the parameters required by the given Description.  If they mismatch in
// either type or number, a fatal error is logged to the given Log, and the
// function returns false; otherwise, no error is logged, and the function
// returns true.
func validateArgs(config *Config, desc *Description, log *Log) bool {
	numArgsExpected := len(desc.Params)
	numArgsSupplied := len(config.Args)
	if numArgsSupplied != numArgsExpected {
		log.Errorf("This refactoring requires %d arguments, "+
			"but %d were supplied.", numArgsExpected,
			numArgsSupplied)
		return false
	}
	for i, arg := range config.Args {
		expected := reflect.TypeOf(desc.Params[i].DefaultValue)
		if reflect.TypeOf(arg) != expected {
			paramName := desc.Params[i].Label
			log.Errorf("%s must be a %s", paramName, expected)
			return false
		}
	}
	return true
}

// lineColToPos converts a line/column position (where the first character in a
// file is at // line 1, column 1) into a token.Pos
func (r *refactoringBase) lineColToPos(file *ast.File, line int, column int) token.Pos {
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

func (r *refactoringBase) checkForErrors() {
	contents, err := ioutil.ReadFile(r.filename(r.file))
	if err != nil {
		r.Log.Errorf("Unable to read source file: %v", err)
		return
	}
	sourceFromFile := string(contents)

	string, err := text.ApplyToString(r.Edits[r.filename(r.file)], sourceFromFile)
	if err != nil {
		r.Log.Errorf("Transformation produced invalid EditSet: %v",
			err.Error())
		return
	}

	_, err = parser.ParseFile(r.program.Fset, "", string, parser.ParseComments)
	if err != nil {
		fmt.Println("vvvvv")
		fmt.Println(string)
		fmt.Println("^^^^^")
		r.Log.Errorf("Transformation will introduce syntax errors: %v", err)
		r.Log.Associate(r.filename(r.file))
		return
	}

	/*
		// TODO: This may be wrong if several files are changed...?
		r.pkgInfo = r.program.CreatePackage(r.file.Name.Name, f)
		if r.pkgInfo.Err != nil {
			r.Log.Log(ERROR, "Transformation will introduce semantic "+
				"errors: "+r.pkgInfo.Err.Error())
			return
		}
	*/
}

/*
//find occurrences of [top level] identifier across package
//TODO filter file? /dumbfounded
func (r *refactoringBase) findAnyOccurrences(name string) bool {
	result := false
	//ast.Inspect(r.file, func(n ast.Node) bool {
	//switch thisIdent := n.(type) {
	//case *ast.Ident:
	//if r.pkgInfo.ObjectOf(thisIdent) == decl {
	//offset := r.program.Fset.Position(thisIdent.NamePos).Offset
	//length := utf8.RuneCountInString(thisIdent.Name)
	//result = append(result, text.OffsetLength{offset, length})
	//}
	//}

	//return true
	//})
	return result
}
*/

/* -=-=- Utility Methods -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

func InterpretArgs(args []string, params []Parameter) []interface{} {
	result := []interface{}{}
	for i, opt := range args {
		if i < len(params) && params[i].IsBoolean() {
			switch opt {
			case "true":
				result = append(result, true)
			case "false":
				result = append(result, false)
			default:
				result = append(result, opt)
			}
		} else {
			result = append(result, opt)
		}
	}
	return result
}

func (r *refactoringBase) pkgInfo(file *ast.File) *loader.PackageInfo {
	for _, pkgInfo := range r.program.AllPackages {
		for _, thisFile := range pkgInfo.Files {
			if thisFile == file {
				return pkgInfo
			}
		}
	}
	return nil
}

func (r *refactoringBase) filename(file *ast.File) string {
	return r.program.Fset.Position(file.Package).Filename
}

func (r *refactoringBase) fileContaining(node ast.Node) *ast.File {
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

func (r *refactoringBase) fileNamed(filename string) (*loader.PackageInfo, *ast.File) {
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

func (r *refactoringBase) forEachFile(f func(ast *ast.File)) {
	for _, pkgInfo := range r.program.AllPackages {
		for _, ast := range pkgInfo.Files {
			f(ast)
		}
	}
}

func (r *refactoringBase) forEachInitialFile(f func(ast *ast.File)) {
	for _, pkgInfo := range r.program.InitialPackages() {
		for _, ast := range pkgInfo.Files {
			f(ast)
		}
	}
}

func (r *refactoringBase) offsetLength(node ast.Node) (int, int) {
	return r.program.Fset.Position(node.Pos()).Offset, (r.program.Fset.Position(node.End()).Offset - r.program.Fset.Position(node.Pos()).Offset)
}

func (r *refactoringBase) lhsNames(assign *ast.AssignStmt) []bytes.Buffer {
	var lhsbuf bytes.Buffer
	buf := make([]bytes.Buffer, len(assign.Lhs))
	for i, lhs := range assign.Lhs {
		offset, length := r.offsetLength(lhs)
		lhsText := r.fileContents[offset : offset+length]
		if len(assign.Lhs) == len(assign.Rhs) {
			buf[i].Write(lhsText)
		} else {
			lhsbuf.Write(lhsText)
			if i < len(assign.Lhs)-1 {
				lhsbuf.WriteString(", ")
			}
			buf[0] = lhsbuf
		}
	}
	return buf
}
