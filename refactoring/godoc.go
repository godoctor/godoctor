// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// This file uses the Null refactoring as a template, and
// adds comments to a program.
// It will be for any program that needs checking for documentation.
package refactoring

import (
	"go/ast"

	"golang-refactoring.org/go-doctor/text"
)

type addGodocRefactoring struct {
	refactoringBase
}

// creates a description of the program
func (r *addGodocRefactoring) Description() *Description {
	return &Description{
		Name:    "Add Godocs",
		Params:  nil,
		Quality: Development,
	}
}

// the base run function needed by most if not all refactorings, checks
// for errors before running this programs coding
func (r *addGodocRefactoring) Run(config *Config) *Result {
	r.refactoringBase.Run(config)
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}
	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}
	r.searchAST()
	return &r.Result
}
func (r *addGodocRefactoring) searchAST() {
	r.removeSemicolonsBetweenDecls()
	r.addComments()
}

// loop through the decls slice to look at all the
// functions, structs, and interfaces
func (r *addGodocRefactoring) removeSemicolonsBetweenDecls() {
	for i := 0; i < len(r.file.Decls)-1; i++ {
		// see if the 2 nodes are on the same line,
		// and if so, run the add in the if statement
		if r.program.Fset.Position(r.file.Decls[i].Pos()).Line == r.program.Fset.Position(r.file.Decls[i+1].Pos()).Line {
			// inserts 2 new lines to separate funcs, structs,
			// and interfaces and get rid of the semicolon
			r.Edits[r.filename(r.file)].Add(text.OffsetLength{r.program.Fset.Position(r.file.Decls[i].End()).Offset, r.program.Fset.Position(r.file.Decls[i+1].Pos()).Offset - r.program.Fset.Position(r.file.Decls[i].End()).Offset}, "\n\n")
		}
	}
}

// loop through the ast Decls and check their Doc section to see if they have comments
func (r *addGodocRefactoring) addComments() {
	for _, n := range r.file.Decls {
		switch x := n.(type) {
		// check the funcs for the comments
		// that appear above the functions (documentation)
		case *ast.FuncDecl:
			fcomment := "// " + x.Name.Name + " TODO: FUNC NEEDS COMMENT INFO\n"
			startOfLine := r.program.Fset.Position(x.Pos()).Offset
			if ast.IsExported(x.Name.Name) {
				if x.Doc == nil {
					r.Edits[r.filename(r.file)].Add(text.OffsetLength{startOfLine, 0}, fcomment)
				}
			}
		// check the structs/interfaces for the comments
		// that appear above structs and interfaces (documentation)
		case *ast.GenDecl:
			startOfLine := r.program.Fset.Position(x.Pos()).Offset
			aList := x.Specs
			for i, _ := range aList {
				// if it's a typespec, then it's a struct, interface,
				// or possibly a func, and since it's one of those,
				// check to see if it has comments
				if spec, ok := aList[i].(*ast.TypeSpec); ok {
					sIcomment := "// " + spec.Name.Name + " TODO: STRUCT/INTERFACE NEEDS COMMENT INFO\n"
					if ast.IsExported(spec.Name.Name) {
						// check if the comment section of the
						// struct or interface is missing comments
						if x.Doc == nil {
							r.Edits[r.filename(r.file)].Add(text.OffsetLength{startOfLine, 0}, sIcomment)
						}
					}
				}
			}
		}
	}
}
