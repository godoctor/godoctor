package doctor

import (
//	"fmt"
//	"go/ast"
//	"go/token"
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
type NullRefactoring struct {
	RefactoringBase
}

func (r *NullRefactoring) Name() string {
	return "Null Refactoring"
}

func (r *NullRefactoring) Configure(args []string) bool {
	return true
}

func (r *NullRefactoring) GetParams() []string {
	return []string{}
}

func (r *NullRefactoring) Run() {
	if r.selectedNode == nil {
		r.log.Log(FATAL_ERROR, "selection cannot be null")
		return // SetSelection did not succeed
	}

	// Just for example
	//r.forEachFile(func (file *token.File, ast *ast.File) {
	//	fmt.Println("Found file", file.Name())
	//})

	r.checkForErrors()
	return
}
