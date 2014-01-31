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

type ReverseAssignRefactoring struct {
	RefactoringBase
}

func (r *ReverseAssignRefactoring) Name() string {
	return "Reverse Assignment Refactoring"
}

func (r *ReverseAssignRefactoring) Configure(args []string) bool {
	return true
}

func (r *ReverseAssignRefactoring) GetParams() []string {
	return nil
}

func (r *ReverseAssignRefactoring) Run() {
	if r.selectedNode == nil {
		r.log.Log(FATAL_ERROR, "selection cannot be null")
		return // SetSelection did not succeed
	}
	switch selectedNode := r.selectedNode.(type) {
	case *ast.GenDecl:
		r.callEditset(selectedNode)
	default:
		r.log.Log(FATAL_ERROR, fmt.Sprintf("Select a short assignment (:=) statement! Selected node is %s", reflect.TypeOf(r.selectedNode)))
	}
	r.checkForErrors()
}

func (r *ReverseAssignRefactoring) lhsNames(decl *ast.GenDecl) string {
	offset, _ := r.offsetLength(decl.Specs[0].(*ast.ValueSpec))
	endOffset := r.program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Names[len(decl.Specs[0].(*ast.ValueSpec).Names)-1].End()).Offset
	return r.readFromFile(offset, (endOffset - offset))
}

// returns the replacement string
func (r *ReverseAssignRefactoring) replacement(decl *ast.GenDecl) string {
	return (fmt.Sprintf("%s := ", r.lhsNames(decl)))
}

//calls the edit set
func (r *ReverseAssignRefactoring) callEditset(decl *ast.GenDecl) {
	start, _ := r.offsetLength(decl)
	repstrlen := r.program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Values[0].Pos()).Offset - r.program.Fset.Position(decl.Pos()).Offset
	r.editSet[r.filename(r.file)].Add(OffsetLength{start, repstrlen}, r.replacement(decl))
}
