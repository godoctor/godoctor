// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a refactoring to rename variables, functions, methods,
// types, interfaces, and packages.

package refactoring

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"code.google.com/p/go.tools/go/loader"

	"golang-refactoring.org/go-doctor/analysis/names"
	"golang-refactoring.org/go-doctor/filesystem"
	"golang-refactoring.org/go-doctor/text"
)

// Rename is a refactoring that changes the names of variables, functions,
// methods, types, interfaces, and packages in Go programs.  It attempts to
// prevent name changes that will introduce syntactic or semantic errors into
// the program.
type Rename struct {
	refactoringBase
	newName string // New name to be given to the selected identifier
}

func (r *Rename) Description() *Description {
	return &Description{
		Name:      "Rename",
		Synopsis:  "Changes the name of an identifier",
		Usage:     "<new_name>",
		Multifile: true,
		Params: []Parameter{Parameter{
			Label:        "New Name:",
			Prompt:       "What to rename this identifier to.",
			DefaultValue: "",
		}},
		Quality: Testing,
	}
}

func (r *Rename) Run(config *Config) *Result {
	r.refactoringBase.Run(config)
	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}

	if r.selectedNode == nil {
		r.Log.Error("Please select an identifier to rename.")
		r.Log.AssociatePos(r.selectionStart, r.selectionEnd)
		return &r.Result
	}

	r.newName = config.Args[0].(string)
	if r.newName == "" {
		r.Log.Error("newName cannot be empty")
		return &r.Result
	}
	if !isIdentifierValid(r.newName) {
		r.Log.Errorf("The new name \"%s\" is not a valid Go identifier", r.newName)
		return &r.Result
	}
	if isReservedWord(r.newName) {
		r.Log.Errorf("The new name \"%s\" is a reserved word", r.newName)
		return &r.Result
	}

	switch ident := r.selectedNode.(type) {
	case *ast.Ident:
		// FIXME: Check if main function (not type/var/etc.) -JO
		if ident.Name == "main" && r.pkgInfo(r.fileContaining(ident)).Pkg.Name() == "main" {
			r.Log.Error("The \"main\" function in the \"main\" package cannot be renamed: it will eliminate the program entrypoint")
			r.Log.AssociateNode(ident)
			return &r.Result
		}

		if ast.IsExported(ident.Name) && !ast.IsExported(r.newName) {
			r.Log.Warn("Renaming an exported name to an unexported name will introduce errors outside the package in which it is declared.")
		}

		r.rename(ident, r.selectedNodePkg)
		r.updateLog(config, false)
		return &r.Result

	case *ast.BasicLit:
		// FIXME: This seems too broad?  -JO
		for pkg, _ := range r.program.AllPackages {
			if pkg.Name() == strings.Replace(ident.Value, "\"", "", 2) {
				searchResult := names.FindReferencesToPackage(pkg.Name(), r.program)
				r.addOccurrences(searchResult)
				r.addFileSystemChanges(searchResult, pkg.Name())
			}
		}
		r.updateLog(config, false)
		return &r.Result

	default:
		r.Log.Error("Please select an identifier to rename.")
		r.Log.AssociatePos(r.selectionStart, r.selectionEnd)
		return &r.Result
	}
}

func isIdentifierValid(newName string) bool {
	b, _ := regexp.MatchString("^\\p{L}[\\p{L}\\p{N}]*$", newName)
	return b
}

func isReservedWord(newName string) bool {
	b, _ := regexp.MatchString("^(break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)$", newName)
	return b
}

func (r *Rename) rename(ident *ast.Ident, pkgInfo *loader.PackageInfo) {
	if conflict := names.FindConflict(pkgInfo.ObjectOf(ident), r.newName); conflict != nil {
		r.Log.Errorf("Renaming %s to %s may cause conflicts with an existing declaration", ident.Name, r.newName)
		r.Log.AssociatePos(conflict.Pos(), conflict.Pos())
	}

	var searchResult map[string][]text.Extent
	if isPackageName(ident, pkgInfo) {
		searchResult = names.FindReferencesToPackage(ident.Name, r.program)
	} else if ts := r.selectedTypeSwitchVar(); ts != nil {
		searchResult = r.extents(names.FindTypeSwitchVarOccurrences(ts, r.selectedNodePkg, r.program), r.program.Fset)
	} else {
		searchResult = r.extents(names.FindOccurrences(pkgInfo.ObjectOf(ident), r.program), r.program.Fset)
	}

	for fname, _ := range searchResult {
		_, file := r.fileNamed(fname)
		occs := names.FindInComments(ident.Name, file, r.program.Fset)
		searchResult[fname] = append(searchResult[fname], occs...)
	}

	r.addOccurrences(searchResult)
	if isPackageName(ident, pkgInfo) {
		r.addFileSystemChanges(searchResult, ident.Name)
	}

	return
}

func isPackageName(ident *ast.Ident, pkgInfo *loader.PackageInfo) bool {
	obj := pkgInfo.ObjectOf(ident)
	if pkgInfo.Pkg.Name() == ident.Name && obj == nil {
		return true
	}

	return false
}

func (r *Rename) selectedTypeSwitchVar() *ast.TypeSwitchStmt {
	obj := r.selectedNodePkg.ObjectOf(r.selectedNode.(*ast.Ident))

	for _, n := range r.pathEnclosingSelection {
		if typeSwitch, ok := n.(*ast.TypeSwitchStmt); ok {
			if asgt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
				if len(asgt.Lhs) == 1 &&
					asgt.Tok == token.DEFINE &&
					asgt.Lhs[0] == r.selectedNode {
					return typeSwitch
				}
			}
			for _, stmt := range typeSwitch.Body.List {
				cc := stmt.(*ast.CaseClause)
				if r.selectedNodePkg.Implicits[cc] == obj {
					return typeSwitch
				}
			}
		}
	}
	return nil
}

func (r *Rename) addOccurrences(allOccurrences map[string][]text.Extent) {
	hasOccsInGoRoot := false
	for filename, occurrences := range allOccurrences {
		if isInGoRoot(filename) {
			hasOccsInGoRoot = true
		} else {
			for _, occurrence := range occurrences {
				if r.Edits[filename] == nil {
					r.Edits[filename] = text.NewEditSet()
				}
				r.Edits[filename].Add(occurrence, r.newName)
			}
		}
	}
	if hasOccsInGoRoot {
		r.Log.Warnf("Occurrences were found in files under $GOROOT, but these will not be renamed")
	}
}

func isInGoRoot(absPath string) bool {
	goRoot := runtime.GOROOT()
	if !strings.HasSuffix(goRoot, string(filepath.Separator)) {
		goRoot += string(filepath.Separator)
	}
	return strings.HasPrefix(absPath, goRoot)
}

func (r *Rename) addFileSystemChanges(allOccurrences map[string][]text.Extent, identName string) {
	for filename, _ := range allOccurrences {

		if filepath.Base(filepath.Dir(filename)) == identName && allFilesinDirectoryhaveSamePkg(filepath.Dir(filename), identName) {
			chg := &filesystem.Rename{filepath.Dir(filename), r.newName}
			r.FSChanges = append(r.FSChanges, chg)
		}
	}
}

func allFilesinDirectoryhaveSamePkg(directorypath string, identName string) bool {
	var renamefile bool = false
	fileInfos, _ := ioutil.ReadDir(directorypath)

	// FIXME: This seems expensive -- is it really necessary?  -JO
	for _, file := range fileInfos {
		if strings.HasSuffix(file.Name(), ".go") {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, filepath.Join(directorypath, file.Name()), nil, 0)
			if err != nil {
				panic(err)
			}
			if f.Name.Name == identName {
				renamefile = true
			}
		}
	}

	return renamefile
}
