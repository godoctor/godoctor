package doctor

import (
	//"fmt"
	"go/ast"
	"reflect"
)

// This file defines the Null refactoring, which makes no changes to a program.

// A NullRefactoring makes no changes to a Go program; it is for testing only.
// It implements the Refactoring interface.
//
// To run the null refactoring:
// * Create a NullRefactoring.
// * Invoke SetSelection to determine what file to refactor.
// * Invoke Run to construct the EditSet.
// * Invoke GetResult to get the resulting Log and EditSet.
//
type ShortAssignRefactoring struct {
	RefactoringBase
}

func (r *ShortAssignRefactoring) Name() string {
	return "Short Assignment Refactoring"
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
				r.editSet.Add(r.filename, OffsetLength{ startOffset, length }, replacement)

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
		r.log.Log(FATAL_ERROR, "(the selected node is " + reflect.TypeOf(r.selectedNode).String() + ")")
		return
	}

	r.checkForErrors()
	return
}
