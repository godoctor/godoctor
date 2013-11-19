// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Null refactoring, which makes no changes to a program.
// It is for testing only (and can be used as a template for building new
// refactorings).

// Contributors: Jeff Overbey

package doctor

import (
//	"fmt"
//	"go/ast"
//	"go/token"
)

// The NullRefactoring makes no changes to a program.
//
// Like all refactorings, it implements the Refactoring interface.
//
// To run the null refactoring:
// * Create a NullRefactoring object.
// * Invoke SetSelection to determine what file to refactor.
// * Invoke Run to construct the EditSets.
// * Invoke GetResult to get the resulting Log and EditSets.
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

	// To display the abstract syntax tree for the selected file:
	//ast.Print(r.importer.Fset, r.file)

	// To iterate through all of the files in the current package:
	//r.forEachFile(func (file *token.File, ast *ast.File) {
	//	fmt.Println("Found file", file.Name())
	//})

	// If there were any semantic errors present in the original file(s),
	// you can downgrade those to warnings as follows:
	r.log.ChangeInitialErrorsToWarnings()
	// or you can remove them altogether using
	//r.log.RemoveInitialEntries()

	// If there were no initial errors, you can check whether or not your
	// refactoring introduced new syntactic or semantic errors as follows:
	//r.checkForErrors()
}
