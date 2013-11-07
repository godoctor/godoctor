// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Contributors: Steffi Gnanaprakasa, Jeff Overbey

package doctor

import (
	//"fmt"
	"go/ast"
	"reflect"
)

// A ShortAssignmentRefactoring changes short assignment statements (n := 5)
// into explicitly-typed variable declarations (var n int = 5).
type ShortAssignRefactoring struct {
	RefactoringBase
}

func (r *ShortAssignRefactoring) Name() string {
	return "Short Assignment Refactoring"
}

func (r *ShortAssignRefactoring) GetParams() []string {
	return []string{}
}

func (r *ShortAssignRefactoring) Configure(args []string) bool {
	return true
}

func (r *ShortAssignRefactoring) Run() {
	if r.selectedNode == nil {
		r.log.Log(FATAL_ERROR, "selection cannot be null")
		return // SetSelection did not succeed
	}

	//ast.Print(r.importer.Fset, r.file)

	switch selectedNode := r.selectedNode.(type) {
	case *ast.AssignStmt:
		// TODO: Make sure it's :=
		if len(selectedNode.Lhs) == 1 && len(selectedNode.Rhs) == 1 {
			startOffset := r.importer.Fset.Position(selectedNode.Pos()).Offset
			endOffset := r.importer.Fset.Position(selectedNode.TokPos).Offset + len(":=")
			length := endOffset - startOffset

			switch lhs := selectedNode.Lhs[0].(type) {
			case *ast.Ident:
				rhsExpr := selectedNode.Rhs[0]
				//fmt.Println("Type of selected node's RHS is ", r.pkgInfo.TypeOf(rhsExpr))

				replacement := "var " + lhs.Name + " " + r.pkgInfo.TypeOf(rhsExpr).String() + " ="
				r.editSet.Add(r.filename, OffsetLength{startOffset, length}, replacement)

				r.checkForErrors()
				return

			default:
				r.log.Log(FATAL_ERROR, "Left-hand side is not an identifier")
			}
		} else {
			r.log.Log(FATAL_ERROR, "Cannot handle multiple assignment")
		}
		return

	default:
		r.log.Log(FATAL_ERROR, "Please select a short assignment (:=) statement")
		r.log.Log(FATAL_ERROR, "(the selected node is "+reflect.TypeOf(r.selectedNode).String()+")")
		return
	}

	r.checkForErrors()
	return
}
