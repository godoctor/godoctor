// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

// This file defines a refactoring to rename variables, functions, methods, structs, and interfaces.
// (TODO: It cannot yet rename packages.)

import (
	"go/ast"
	"regexp"

	"code.google.com/p/go.tools/go/types"
)

// A RenameRefactoring is used to rename identifiers in Go programs.
type RenameRefactoring struct {
	RefactoringBase
	newName   string
	signature *types.Signature
}

func (r *RenameRefactoring) Name() string {
	return "Rename"
}

func (r *RenameRefactoring) SetNewName(newName string) {

	if r.isIdentifierValid(newName) {
		r.newName = newName
	} else {
		r.log.Log(FATAL_ERROR, "Please select a valid Go identifier")
	}
}

func (r *RenameRefactoring) GetParams() []string {
	return []string{"New Name"}
}

func (r *RenameRefactoring) Configure(args []string) bool {
	if len(args) == 1 {
		r.SetNewName(args[0])
		return true
	} else {
		r.log.Log(FATAL_ERROR, "(Internal Error) Invalid arguments")
		return false
	}
}

func (r *RenameRefactoring) Run() {
	if r.selectedNode == nil {
		r.log.Log(FATAL_ERROR, "Please select an identifier to rename.")
		return
	}

	if r.newName == "" {
		r.log.Log(FATAL_ERROR, "newName cannot be empty")
		return
	}

	switch ident := r.selectedNode.(type) {
	case *ast.Ident:
		r.rename(ident)

	default:
		r.log.Log(FATAL_ERROR, "Please select an identifier to rename.")
		return
	}
}

func (r *RenameRefactoring) isIdentifierValid(newName string) bool {

	matched, err := regexp.MatchString("^[A-Za-z_][0-9A-Za-z_]*$", newName)
	if matched && err == nil {
		return true
	}
	return false
}

func (r *RenameRefactoring) rename(ident *ast.Ident) {

	if !r.IdentifierExists(ident) {
		search := &SearchEngine{r.program}
		searchResult, err := search.FindOccurrences(ident)
		if err != nil {
			r.log.Log(FATAL_ERROR, err.Error())
			return
		}

		r.addOccurrences(searchResult)
		//TODO: r.checkForErrors()
		return
	} 

      r.log.Log(FATAL_ERROR, "newname already exists in scope,please select other value for the newname")

}

//IdentifierExists checks if there already exists an Identifier with the newName,with in the scope of the oldname.
func (r *RenameRefactoring) IdentifierExists(ident *ast.Ident) bool {

	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)

	if obj == nil {
		r.log.Log(FATAL_ERROR, "unable to find declaration of selected identifier")
		return false
	}

	identscope := obj.Parent()

	if identscope.LookupParent(r.newName) != nil {
		return true
	}

	return false
}


//addOccurrences adds all the Occurences to the editset
func (r *RenameRefactoring) addOccurrences(allOccurrences map[string][]OffsetLength) {
	for filename, occurrences := range allOccurrences {
		for _, occurrence := range occurrences {
			if r.editSet[filename] == nil {
				r.editSet[filename] = NewEditSet()
			}
			r.editSet[filename].Add(occurrence, r.newName)
		}

	}

}
