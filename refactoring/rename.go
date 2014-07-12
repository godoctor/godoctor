// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a refactoring to rename variables, functions, methods,
// structs, interfaces, and packages.

package refactoring

import (
	"go/ast"
	"regexp"
	"runtime"
	//"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"

	"code.google.com/p/go.tools/go/types"
	"golang-refactoring.org/go-doctor/analysis/names"
	"golang-refactoring.org/go-doctor/filesystem"
	"golang-refactoring.org/go-doctor/text"
)

// A Rename refactoring is used to rename identifiers in Go programs.
type Rename struct {
	refactoringBase
	newName   string
	signature *types.Signature
}

func (r *Rename) Description() *Description {
	return &Description{
		Name: "Rename",
		Params: []Parameter{Parameter{
			Label:        "New Name:",
			Prompt:       "What to rename this identifier to.",
			DefaultValue: "",
		}},
		Quality: Development,
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

	r.newName = config.Args[0].(string)
	if !r.isIdentifierValid(r.newName) {
		r.Log.Errorf("The new name %s is not a valid Go identifier", r.newName)
		return &r.Result
	}

	if r.selectedNode == nil {
		r.Log.Error("Please select an identifier to rename.")
		r.Log.AssociatePos(r.selectionStart, r.selectionEnd)
		return &r.Result
	}

	if r.newName == "" {
		r.Log.Error("newName cannot be empty")
		return &r.Result
	}

	switch ident := r.selectedNode.(type) {
	case *ast.Ident:
		if ast.IsExported(ident.Name) && !ast.IsExported(r.newName) {
			r.Log.Error("newName cannot be non Exportable if selected identifier name is Exportable")
			return &r.Result
		}
		if ident.Name == "main" && r.pkgInfo(r.fileContaining(ident)).Pkg.Name() == "main" {
			r.Log.Error("cannot rename main function inside main package ,it eliminates the program entry 							point")
			r.Log.AssociateNode(ident)
			return &r.Result
		}

		r.rename(ident)
		r.updateLog(config, false)
	default:
		r.Log.Error("Please select an identifier to rename.")
		r.Log.AssociatePos(r.selectionStart, r.selectionEnd)
	}
	return &r.Result
}

func (r *Rename) isIdentifierValid(newName string) bool {
	matched, err := regexp.MatchString("^\\p{L}[\\p{L}\\p{N}]*$", newName)
	if matched && err == nil {
		keyword, err := regexp.MatchString("^(break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)$", newName)
		return !keyword && err == nil
	}
	return false
}

func (r *Rename) rename(ident *ast.Ident) {
	if !r.identExists(ident) {
		search := names.NewSearchEngine(r.program)
		searchResult, err := search.FindOccurrences(ident)
		if err != nil {
			r.Log.Error(err)
			return
		}

		r.addOccurrences(searchResult)
		if search.IsPackageName(ident) {
			r.addFileSystemChanges(searchResult, ident)
		}
		//TODO: r.checkForErrors()
		return
	}

}

//IdentifierExists checks if there already exists an Identifier with the newName,with in the scope of the oldname.
func (r *Rename) identExists(ident *ast.Ident) bool {

	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)
	search := names.NewSearchEngine(r.program)

	if obj == nil && !search.IsPackageName(ident) {

		r.Log.Error("unable to find declaration of selected identifier")
		r.Log.AssociateNode(ident)
		return true
	}

	if search.IsPackageName(ident) {
		return false
	}
	identscope := obj.Parent()

	if names.IsMethod(obj) {
		objfound, _, pointerindirections := types.LookupFieldOrMethod(names.MethodReceiver(obj).Type(), obj.Pkg(), r.newName)
		if names.IsMethod(objfound) && pointerindirections {
			r.Log.Error("newname already exists in scope,please select other value for the newname")
			r.Log.AssociateNode(ident)
			return true
		} else {
			return false
		}
	}

	if identscope.LookupParent(r.newName) != nil {
		r.Log.Error("newname already exists in scope,please select other value for the newname")
		r.Log.AssociateNode(ident)
		return true
	}

	return false
}

//addOccurrences adds all the Occurences to the editset
func (r *Rename) addOccurrences(allOccurrences map[string][]text.Extent) {
	for filename, occurrences := range allOccurrences {
		if isInGoRoot(filename) {
			r.Log.Warnf("Occurrences in %s will not be renamed",
				filename)
		} else {
			for _, occurrence := range occurrences {
				if r.Edits[filename] == nil {
					r.Edits[filename] = text.NewEditSet()
				}
				r.Edits[filename].Add(occurrence, r.newName)
			}
		}
	}
}

func isInGoRoot(absPath string) bool {
	return strings.HasPrefix(absPath, runtime.GOROOT())
}

func (r *Rename) addFileSystemChanges(allOccurrences map[string][]text.Extent, ident *ast.Ident) {
	for filename, _ := range allOccurrences {

		if filepath.Base(filepath.Dir(filename)) == ident.Name && allFilesinDirectoryhaveSamePkg(filepath.Dir(filename), ident) {
			chg := &filesystem.Rename{filepath.Dir(filename), r.newName}
			r.FSChanges = append(r.FSChanges, chg)
		}
	}
}

func allFilesinDirectoryhaveSamePkg(directorypath string, ident *ast.Ident) bool {

	var renamefile bool = false
	fileInfos, _ := ioutil.ReadDir(directorypath)

	for _, file := range fileInfos {
		if strings.HasSuffix(file.Name(), ".go") {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, filepath.Join(directorypath, file.Name()), nil, 0)
			if err != nil {
				panic(err)
			}
			if f.Name.Name == ident.Name {
				renamefile = true
			}
		}
	}

	return renamefile
}
