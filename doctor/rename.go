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

// A renameRefactoring is used to rename identifiers in Go programs.
type renameRefactoring struct {
	refactoringBase
	newName   string
	signature *types.Signature
}

func (r *renameRefactoring) Description() *Description {
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

func (r *renameRefactoring) Run(config *Config) *Result {
	if r.refactoringBase.Run(config); r.Log.ContainsErrors() {
		return &r.Result
	}

	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	r.newName = config.Args[0].(string)
	if !r.isIdentifierValid(r.newName) {
		r.Log.Log(FATAL_ERROR, "The new name "+r.newName+" is not a valid Go identifier")
		return &r.Result
	}

	if r.selectedNode == nil {
		r.Log.Log(FATAL_ERROR, "Please select an identifier to rename.")
		return &r.Result
	}

	if r.newName == "" {
		r.Log.Log(FATAL_ERROR, "newName cannot be empty")
		return &r.Result
	}

	switch ident := r.selectedNode.(type) {
	case *ast.Ident:
		r.rename(ident)

	default:
		r.Log.Log(FATAL_ERROR, "Please select an identifier to rename.")
	}
	return &r.Result
}

func (r *renameRefactoring) isIdentifierValid(newName string) bool {
	matched, err := regexp.MatchString("^[A-Za-z_][0-9A-Za-z_]*$", newName)
	if matched && err == nil {
		keyword, err := regexp.MatchString("^(break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)$", newName)
		return !keyword && err == nil
	}
	return false
}

func (r *renameRefactoring) rename(ident *ast.Ident) {

	if !r.IdentifierExists(ident) {
		search := &SearchEngine{r.program}
		searchResult, err := search.FindOccurrences(ident)
		if err != nil {
			r.Log.Log(FATAL_ERROR, err.Error())
			return
		}

		r.addOccurrences(searchResult)
		//TODO: r.checkForErrors()
		return
	}

	r.Log.Log(FATAL_ERROR, "newname already exists in scope,please select other value for the newname")

}

//IdentifierExists checks if there already exists an Identifier with the newName,with in the scope of the oldname.
func (r *renameRefactoring) IdentifierExists(ident *ast.Ident) bool {

	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)

	if obj == nil {
		r.Log.Log(FATAL_ERROR, "unable to find declaration of selected identifier")
		return false
	}

	identscope := obj.Parent()

	if identscope.LookupParent(r.newName) != nil {
		return true
	}

	return false
}

//addOccurrences adds all the Occurences to the editset
func (r *renameRefactoring) addOccurrences(allOccurrences map[string][]OffsetLength) {
	for filename, occurrences := range allOccurrences {
		for _, occurrence := range occurrences {
			if r.Edits[filename] == nil {
				r.Edits[filename] = NewEditSet()
			}
			r.Edits[filename].Add(occurrence, r.newName)
		}

	}

}
