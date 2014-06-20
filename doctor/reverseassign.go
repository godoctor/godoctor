// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A ReverseAssignmentRefactoring changes explicitly-typed variable declarations (var n int = 5)
//into short assignment statements (n := 5)

package doctor

import (
	"fmt"
	"go/ast"
	"reflect"
)

type reverseAssignRefactoring struct {
	refactoringBase
}

func (r *reverseAssignRefactoring) Description() *Description {
	return &Description{
		Name:    "Reverse Assignment Refactoring",
		Params:  nil,
		Quality: Development,
	}
}

func (r *reverseAssignRefactoring) Run(config *Config) *Result {
	if r.refactoringBase.Run(config); r.Log.ContainsErrors() {
		return &r.Result
	}

	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	if r.selectedNode == nil {
		r.Log.Log(FATAL_ERROR, "selection cannot be null")
		return &r.Result
	}
	switch selectedNode := r.selectedNode.(type) {
	case *ast.GenDecl:
		r.callEditset(selectedNode)
	default:
		r.Log.Log(FATAL_ERROR, fmt.Sprintf("Select a short assignment (:=) statement! Selected node is %s", reflect.TypeOf(r.selectedNode)))
	}
	r.checkForErrors()
	return &r.Result
}

func (r *reverseAssignRefactoring) lhsNames(decl *ast.GenDecl) string {
	offset, _ := r.offsetLength(decl.Specs[0].(*ast.ValueSpec))
	endOffset := r.program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Names[len(decl.Specs[0].(*ast.ValueSpec).Names)-1].End()).Offset
	return string(r.fileContents[offset:endOffset])
}

// returns the replacement string
func (r *reverseAssignRefactoring) replacement(decl *ast.GenDecl) string {
	return (fmt.Sprintf("%s := ", r.lhsNames(decl)))
}

//calls the edit set
func (r *reverseAssignRefactoring) callEditset(decl *ast.GenDecl) {
	start, _ := r.offsetLength(decl)
	repstrlen := r.program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Values[0].Pos()).Offset - r.program.Fset.Position(decl.Pos()).Offset
	r.Edits[r.filename(r.file)].Add(OffsetLength{start, repstrlen}, r.replacement(decl))
}
