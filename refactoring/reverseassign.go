// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a refactoring that converts explicitly-typed variable
// declarations (var n int = 5) into short assignment statements (n := 5).

package refactoring

import (
	"fmt"
	"go/ast"
	"reflect"

	"golang-refactoring.org/go-doctor/text"
)

// A ReverseAssign refactoring changes explicitly-typed variable declarations
// (var n int = 5) into short assignment statements (n := 5).
type ReverseAssign struct {
	refactoringBase
}

func (r *ReverseAssign) Description() *Description {
	return &Description{
		Name:    "Convert var to :=",
		Params:  nil,
		Quality: Development,
	}
}

func (r *ReverseAssign) Run(config *Config) *Result {
	if r.refactoringBase.Run(config); r.Log.ContainsErrors() {
		return &r.Result
	}

	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	if r.selectedNode == nil {
		r.Log.Error("selection cannot be null")
		r.Log.AssociatePos(r.program.Fset, r.selectionStart, r.selectionEnd)
		return &r.Result
	}
	switch selectedNode := r.selectedNode.(type) {
	case *ast.GenDecl:
		r.callEditset(selectedNode)
	default:
		r.Log.Errorf("Select a short assignment (:=) statement! Selected node is %s", reflect.TypeOf(r.selectedNode))
		r.Log.AssociatePos(r.program.Fset, r.selectionStart, r.selectionEnd)
	}
	r.updateLog(config)
	return &r.Result
}

func (r *ReverseAssign) lhsNames(decl *ast.GenDecl) string {
	offset, _ := r.offsetLength(decl.Specs[0].(*ast.ValueSpec))
	endOffset := r.program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Names[len(decl.Specs[0].(*ast.ValueSpec).Names)-1].End()).Offset
	return string(r.fileContents[offset:endOffset])
}

// returns the replacement string
func (r *ReverseAssign) replacement(decl *ast.GenDecl) string {
	return (fmt.Sprintf("%s := ", r.lhsNames(decl)))
}

//calls the edit set
func (r *ReverseAssign) callEditset(decl *ast.GenDecl) {
	start, _ := r.offsetLength(decl)
	repstrlen := r.program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Values[0].Pos()).Offset - r.program.Fset.Position(decl.Pos()).Offset
	r.Edits[r.filename(r.file)].Add(text.Extent{start, repstrlen}, r.replacement(decl))
}
