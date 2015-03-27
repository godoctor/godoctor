// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a refactoring to rename variables, functions, methods,
// types, interfaces, and packages.

package refactoring

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/tools/go/loader"

	"github.com/godoctor/godoctor/analysis/names"
	"github.com/godoctor/godoctor/text"
)

// Rename is a refactoring that changes the names of variables, functions,
// methods, types, interfaces, and packages in Go programs.  It attempts to
// prevent name changes that will introduce syntactic or semantic errors into
// the Program.
type Rename struct {
	base    RefactoringBase
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
		Hidden: false,
	}
}

func (r *Rename) Run(config *Config) *Result {
	r.base.Run(config)
	if !ValidateArgs(config, r.Description(), r.base.Log) {
		return &r.base.Result
	}
	r.base.Log.ChangeInitialErrorsToWarnings()
	if r.base.Log.ContainsErrors() {
		return &r.base.Result
	}

	if r.base.SelectedNode == nil {
		r.base.Log.Error("Please select an identifier to rename.")
		r.base.Log.AssociatePos(r.base.SelectionStart, r.base.SelectionEnd)
		return &r.base.Result
	}

	r.newName = config.Args[0].(string)
	if r.newName == "" {
		r.base.Log.Error("newName cannot be empty")
		return &r.base.Result
	}
	if !isIdentifierValid(r.newName) {
		r.base.Log.Errorf("The new name \"%s\" is not a valid Go identifier", r.newName)
		return &r.base.Result
	}
	if isReservedWord(r.newName) {
		r.base.Log.Errorf("The new name \"%s\" is a reserved word", r.newName)
		return &r.base.Result
	}

	switch ident := r.base.SelectedNode.(type) {
	case *ast.Ident:

		// FIXME: Check if main function (not type/var/etc.) -JO
		if ident.Name == "main" && r.base.SelectedNodePkg.Pkg.Name() == "main" {
			r.base.Log.Error("The \"main\" function in the \"main\" package cannot be renamed: it will eliminate the program entrypoint")
			r.base.Log.AssociateNode(ident)
			return &r.base.Result
		}

		if isPredeclaredIdentifier(ident.Name) {
			r.base.Log.Errorf("selected predeclared  identifier \"%s\" , it cannot be renamed", ident.Name)
			r.base.Log.AssociateNode(ident)
			return &r.base.Result
		}

		if ast.IsExported(ident.Name) && !ast.IsExported(r.newName) {
			r.base.Log.Warn("Renaming an exported name to an unexported name will introduce errors outside the package in which it is declared.")
		}

		r.rename(ident, r.base.SelectedNodePkg)
		r.base.UpdateLog(config, false)
		return &r.base.Result

	default:
		r.base.Log.Errorf("Please select an identifier to rename. "+
			"(Selected node: %s)", reflect.TypeOf(ident))
		r.base.Log.AssociatePos(r.base.SelectionStart, r.base.SelectionEnd)
		return &r.base.Result
	}
}

func isIdentifierValid(newName string) bool {
	b, _ := regexp.MatchString("^[\\p{L}|_][\\p{L}|_|\\p{N}]*$", newName)
	return b
}

func isPredeclaredIdentifier(selectedIdentifier string) bool {
	b, _ := regexp.MatchString("^(bool|byte|complex64|complex128|error|float32|float64|int|int8|int16|int32|int64|rune|string|uint|uint8|uint16|uint32|uint64|uintptr|true|false|iota|nil|append|cap|close|complex|copy|delete|imag|len|make|new|panic|print|println|real|recover)$", selectedIdentifier)
	return b
}

func isReservedWord(newName string) bool {
	b, _ := regexp.MatchString("^(break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)$", newName)
	return b
}

func (r *Rename) rename(ident *ast.Ident, pkgInfo *loader.PackageInfo) {
	obj := pkgInfo.ObjectOf(ident)

	if obj == nil && r.selectedTypeSwitchVar() == nil {
		r.base.Log.Errorf("Package renaming is not supported")
		r.base.Log.AssociateNode(ident)
		return
	}

	if obj != nil && isInGoRoot(r.base.Program.Fset.Position(obj.Pos()).Filename) {
		r.base.Log.Errorf("%s is defined in $GOROOT and cannot be renamed",
			ident.Name)
		r.base.Log.AssociateNode(ident)
		return
	}
	if conflict := names.FindConflict(obj, r.newName); conflict != nil {
		r.base.Log.Errorf("Renaming %s to %s may cause conflicts with an existing declaration", ident.Name, r.newName)
		r.base.Log.AssociatePos(conflict.Pos(), conflict.Pos())
	}
	var idents map[*ast.Ident]bool
	if ts := r.selectedTypeSwitchVar(); ts != nil {
		idents = names.FindTypeSwitchVarOccurrences(ts, r.base.SelectedNodePkg, r.base.Program)
	} else {
		idents = names.FindOccurrences(obj, r.base.Program)
	}

	r.addOccurrences(ident.Name, r.extents(idents, r.base.Program.Fset))
}

func (r *Rename) selectedTypeSwitchVar() *ast.TypeSwitchStmt {
	obj := r.base.SelectedNodePkg.ObjectOf(r.base.SelectedNode.(*ast.Ident))

	for _, n := range r.base.PathEnclosingSelection {
		if typeSwitch, ok := n.(*ast.TypeSwitchStmt); ok {
			if asgt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
				if len(asgt.Lhs) == 1 &&
					asgt.Tok == token.DEFINE &&
					asgt.Lhs[0] == r.base.SelectedNode {
					return typeSwitch
				}
			}
			for _, stmt := range typeSwitch.Body.List {
				cc := stmt.(*ast.CaseClause)
				if r.base.SelectedNodePkg.Implicits[cc] == obj {
					return typeSwitch
				}
			}
		}
	}
	return nil
}

func (r *Rename) extents(ids map[*ast.Ident]bool, fset *token.FileSet) map[string][]*text.Extent {
	result := map[string][]*text.Extent{}
	for id, _ := range ids {
		pos := fset.Position(id.Pos())
		if _, ok := result[pos.Filename]; !ok {
			result[pos.Filename] = []*text.Extent{}
		}
		result[pos.Filename] = append(result[pos.Filename],
			&text.Extent{Offset: pos.Offset, Length: len(id.Name)})
	}

	sorted := map[string][]*text.Extent{}
	for fname, extents := range result {
		sorted[fname] = text.Sort(extents)
	}
	return sorted
}

func (r *Rename) addOccurrences(name string, allOccurrences map[string][]*text.Extent) {
	hasOccsInGoRoot := false
	for filename, occurrences := range allOccurrences {
		if isInGoRoot(filename) {
			hasOccsInGoRoot = true
		} else {
			if r.base.Edits[filename] == nil {
				r.base.Edits[filename] = text.NewEditSet()
			}
			for _, occurrence := range occurrences {
				r.base.Edits[filename].Add(occurrence, r.newName)
			}
			_, file := r.fileNamed(filename)
			commentOccurrences := names.FindInComments(
				name, file, r.base.Program.Fset)
			for _, occurrence := range commentOccurrences {
				r.base.Edits[filename].Add(occurrence, r.newName)
			}
		}
	}
	if hasOccsInGoRoot {
		r.base.Log.Warnf("Occurrences were found in files under $GOROOT, but these will not be renamed")
	}
}

func isInGoRoot(absPath string) bool {
	goRoot := runtime.GOROOT()
	if !strings.HasSuffix(goRoot, string(filepath.Separator)) {
		goRoot += string(filepath.Separator)
	}
	return strings.HasPrefix(absPath, goRoot)
}

func (r *Rename) fileNamed(filename string) (*loader.PackageInfo, *ast.File) {
	absFilename, _ := filepath.Abs(filename)
	for _, pkgInfo := range r.base.Program.AllPackages {
		for _, f := range pkgInfo.Files {
			thisFile := r.base.Program.Fset.Position(f.Pos()).Filename
			if thisFile == filename || thisFile == absFilename {
				return pkgInfo, f
			}
		}
	}
	return nil, nil
}
