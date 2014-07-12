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
	"sync"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
	"golang-refactoring.org/go-doctor/filesystem"
	"golang-refactoring.org/go-doctor/text"
)

// The maximum number of errors from the go/loader that will be reported
const maxInitialErrors = 10

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
	Selection text.Selection
	// Refactoring-specific arguments.  To determine what arguments are
	// required for each refactoring, see Refactoring.Description().Params.
	// For example, for the Rename refactoring, you must specify a new name
	// for the entity being renamed.  If the refactoring does not require
	// any arguments, this may be nil.
	Args []interface{}
	// If true, an exhaustive list of edits made by the refactoring will be
	// appended to the log.
	Verbose bool
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
	r.Edits = map[string]*text.EditSet{}
	r.FSChanges = []filesystem.Change{}

	if config.FileSystem == nil {
		r.Log.Error("INTERNAL ERROR: null Config.FileSystem")
		return &r.Result
	}

	if config.Scope == nil {
		var msg string
		config.Scope, msg = r.guessScope(config)
		r.Log.Infof(msg)
	} else {
		r.Log.Infof("Scope is %s", strings.Join(config.Scope, " "))
	}

	var err error
	mutex := &sync.Mutex{}
	r.program, err = createLoader(config, func(err error) {
		message := err.Error()
		// TODO: This is temporary until go/loader handles cgo
		if !strings.Contains(message, cgoError1) &&
			!strings.HasSuffix(message, cgoError2) &&
			len(r.Log.Entries) < maxInitialErrors {
			mutex.Lock()
			r.Log.Error(message)
			if err, ok := err.(types.Error); ok {
				r.Log.AssociatePos(err.Pos, err.Pos)
			}
			mutex.Unlock()
		}
	})

	r.Log.MarkInitial()
	if err != nil {
		r.Log.Error(err)
		return &r.Result
	} else if r.program == nil {
		r.Log.Error("INTERNAL ERROR: Loader failed")
		return &r.Result
	}

	r.Log.Fset = r.program.Fset

	r.selectionStart, r.selectionEnd, err = config.Selection.Convert(r.program.Fset)
	if err != nil {
		r.Log.Error(err)
		return &r.Result
	}

	pkgInfo, nodes, _ := r.program.PathEnclosingInterval(r.selectionStart, r.selectionEnd)
	if pkgInfo == nil || len(nodes) < 1 {
		r.Log.Errorf("The selected file, %s, was not found in the "+
			"provided scope: %s",
			config.Selection.GetFilename(),
			config.Scope)
		// This can happen on files containing +build
		return &r.Result
	}
	r.selectedNode = nodes[0]
	r.file = nodes[len(nodes)-1].(*ast.File)

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

	r.Edits = map[string]*text.EditSet{
		r.filename(r.file): text.NewEditSet(),
	}

	return &r.Result
}

func createLoader(config *Config, errorHandler func(error)) (*loader.Program, error) {
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
	lconfig.TypeChecker.Error = errorHandler

	rest, err := lconfig.FromArgs(config.Scope, true)
	if len(rest) > 0 {
		errorHandler(fmt.Errorf("Unrecognized argument %s",
			strings.Join(rest, " ")))
	}
	if err != nil {
		errorHandler(err)
	}
	return lconfig.Load()
}

// guessScope makes a reasonable guess at the refactoring scope if the user
// does not provide an explicit scope.  It guesses as follows:
//     1. If filename is not in $GOPATH/src, filename is used as the scope.
//     2. If filename is in $GOPATH/src, a package name is guessed by stripping
//        $GOPATH/src/ from the filename, and that package is used as the scope.
func (r *refactoringBase) guessScope(config *Config) ([]string, string) {
	fname := config.Selection.GetFilename()
	fnameScope := []string{fname}
	fnameMsg := fmt.Sprintf("Defaulting to file scope %s for refactoring (provide an explicit scope to change this)", fname)

	if filepath.Base(fname) == filesystem.FakeStdinFilename {
		return fnameScope, "Defaulting to file scope for refactoring (provide an explicit scope to change this)"
	}

	absFilename, err := filepath.Abs(fname)
	if err != nil {
		r.Log.Error(err.Error())
		return fnameScope, fnameMsg
	}

	gopath := config.GoPath
	if gopath == "" {
		gopath = os.Getenv("GOPATH")
	}
	if gopath == "" {
		r.Log.Warn("GOPATH not set")
		return fnameScope, fnameMsg
	}
	gopath, err = filepath.Abs(gopath)
	if err != nil {
		r.Log.Error(err)
		return fnameScope, fnameMsg
	}

	gopathSrc := filepath.Join(gopath, "src")

	relFilename, err := filepath.Rel(gopathSrc, absFilename)
	if err != nil {
		r.Log.Error(err)
		return fnameScope, fnameMsg
	}

	if strings.HasPrefix(relFilename, "..") {
		return fnameScope, fnameMsg
	}

	dir := filepath.Dir(relFilename)
	if dir == "." {
		return fnameScope, fnameMsg
	}

	pkg := filepath.ToSlash(dir)
	return []string{pkg},
		fmt.Sprintf("Defaulting to package scope %s for refactoring (provide an explicit scope to change this)", pkg)
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

// updateLog applies the edits in r.Edits and updates existing error messages
// in r.Log to reflect their locations in the resulting program.  If
// checkForErrors is true, and if the log does not contain any initial errors,
// the resulting program will be type checked, and any new errors introduced by
// the refactoring will be logged.
func (r *refactoringBase) updateLog(config *Config, checkForErrors bool) {
	if r.Edits == nil || len(r.Edits) == 0 {
		return
	}

	if !config.Verbose && !r.Log.ContainsPositions() && !checkForErrors {
		// No reason to load the program, since we won't update any
		// positions and we won't report any new errors
		return
	}

	if r.Log.ContainsInitialErrors() {
		checkForErrors = false
	}

	oldFS := config.FileSystem
	defer func() { config.FileSystem = oldFS }()
	config.FileSystem = filesystem.NewEditedFileSystem(r.Edits)

	newLogOldPos := NewLog()
	newLogOldPos.Fset = r.program.Fset
	newLogNewPos := NewLog()

	mutex := &sync.Mutex{}
	errors := 0
	newProg, err := createLoader(config, func(err error) {
		if !checkForErrors {
			return
		}
		message := err.Error()
		// TODO: This is temporary until go/loader handles cgo
		if !strings.Contains(message, cgoError1) &&
			!strings.HasSuffix(message, cgoError2) &&
			errors < maxInitialErrors {
			mutex.Lock()
			errors++
			msg := fmt.Sprintf("Completing the transformation will introduce the following error: %s", message)
			newLogOldPos.Error(msg)
			newLogNewPos.Error(msg)
			if err, ok := err.(types.Error); ok {
				oldPos := mapPos(err.Fset, err.Pos, r.Edits, r.program.Fset, true)
				newLogOldPos.AssociatePos(oldPos, oldPos)
				newLogNewPos.Fset = err.Fset
				newLogNewPos.AssociatePos(err.Pos, err.Pos)
			}
			mutex.Unlock()
		}
	})
	if newProg == nil || err != nil {
		r.Log.Append(newLogOldPos.Entries)
		return
	}

	r.Log.Fset = newProg.Fset
	for _, entry := range r.Log.Entries {
		entry.Pos = mapPos(r.program.Fset, entry.Pos, r.Edits, newProg.Fset, false)
	}
	r.Log.Append(newLogNewPos.Entries)

	if config.Verbose {
		fileCount := len(r.Edits)
		fileNum := 1
		for filename, edits := range r.Edits {
			firstEdit := true
			edits.Iterate(func(extent text.Extent, replace string) {
				oldFile := findFile(filename, r.program.Fset)
				oldPos := oldFile.Pos(extent.Offset)
				newPos := mapPos(r.program.Fset, oldPos,
					r.Edits, r.Log.Fset, true)
				if firstEdit && fileCount > 1 {
					r.Log.Infof("File %d of %d: %s",
						fileNum,
						fileCount,
						filepath.Base(filename))
					r.Log.AssociatePos(newPos, newPos)
					fileNum++
					firstEdit = false
				}
				r.Log.Infof(describeEdit(extent, replace))
				r.Log.AssociatePos(newPos, newPos)
			})
		}
	}
}

// describeEdit returns a human-readable, one-line description of a text edit
func describeEdit(extent text.Extent, replacement string) string {
	if extent.Length == 0 {
		return fmt.Sprintf("| Insert \"%s\"", shorten(replacement))
	} else if replacement == "" {
		return fmt.Sprintf("| Delete %d byte(s)", extent.Length)
	} else {
		return fmt.Sprintf("| Replace %d byte(s) with \"%s\"",
			extent.Length, shorten(replacement))
	}
}

func shorten(s string) string {
	if len(s) < 23 {
		return s
	}
	return s[:23] + "..."
}

// mapPos takes a Pos in one FileSet and returns the corresponding Pos in
// another FileSet, applying or undoing the given edits (if reverse is false or
// true, respectively) to determine the corresponding offset and comparing
// filenames (as strings) to find the corresponding file.
func mapPos(from *token.FileSet, pos token.Pos, edits map[string]*text.EditSet, to *token.FileSet, reverse bool) token.Pos {
	if !pos.IsValid() {
		return pos
	}

	filename := from.Position(pos).Filename
	offset := from.Position(pos).Offset
	if es, ok := edits[filename]; ok {
		if reverse {
			offset = es.OldOffset(offset)
		} else {
			offset = es.NewOffset(offset)
		}
	}

	result := token.NoPos
	if file := findFile(filename, to); file != nil {
		result = file.Pos(offset)
	}
	return result
}

// findFile searches the given FileSet for a File with the given name and
// returns it (or nil if no such file could be found).
func findFile(filename string, fset *token.FileSet) *token.File {
	var result *token.File = nil
	fset.Iterate(func(f *token.File) bool {
		if f.Name() == filename {
			result = f
			return false
		}
		return true
	})
	return result
}

/* -=-=- Utility Methods -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

func InterpretArgs(args []string, r Refactoring) []interface{} {
	params := r.Description().Params
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
