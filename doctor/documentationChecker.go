// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// This file uses the Null refactoring as a template, and
// adds comments to a program.
// It will be for any program that needs checking for documentation.
package doctor

import (
	"go/ast"

	"strings"
)

type documentationCheckerRefactoring struct {
	refactoringBase
}

// creates a description of the program
func (r *documentationCheckerRefactoring) Description() *Description {
	return &Description{
		Name:    "Documentation Search For Extractable funcs, structs, and interfaces Refactoring",
		Params:  nil,
		Quality: Development,
	}
}

// the base run function needed by most if not all refactorings, checks
// for errors before running this programs coding
func (r *documentationCheckerRefactoring) Run(config *Config) *Result {
	r.refactoringBase.Run(config)
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}
	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}
	SearchAST(r)
	return &r.Result
}

/* this function is the meat of the program.
   It will search through a give file's ast and find all the exportable
   functions, structs, and interfaces one at a time and will see if they
   have documentation, and if not, will add a comment letting you know
   that you will have to add documentation for the funcs, structs, or
   interfaces (knows if they are exportable by the first letter of the
   func/struct/interface name, as it will be capital if it's exportable.*/
func SearchAST(r *documentationCheckerRefactoring) {
	/* make a slice the length of all the nodes for funcs, structs,
	   and interfaces to hold each node so you can check to see
	   if they are on the same line; technically uses the decls
	   slice from the *ast.file node (root node) and
	   then uses the for loop to run through the*/
	dList := r.file.Decls
	// loop through the decls slice to look at all the
	// functions, structs, and interfaces
	for i := 0; i < len(dList)-1; i++ {
		// see if the 2 nodes are on the same line,
		// and if so, run the add in the if statement
		if r.program.Fset.Position(dList[i].Pos()).Line == r.program.Fset.Position(dList[i+1].Pos()).Line {
			// inserts 2 new lines to separate functions,
			// structs, and interfaces from each other
			// (used to get rid of spaces before and after
			// semicolons, and to get rid of semicolons
			r.Edits[r.filename(r.file)].Add(OffsetLength{r.program.Fset.Position(dList[i].End()).Offset, r.program.Fset.Position(dList[i+1].Pos()).Offset - r.program.Fset.Position(dList[i].End()).Offset}, "\n\n")
		}
	}
	// inspect the ast and search the identifier's for func, struct, and interfaces
	// after finding, check their Doc section to see if they have comments
	ast.Inspect(r.file, func(n ast.Node) bool {
		// string for the name of the node
		var s string
		switch x := n.(type) {
		// check all the funcdecl nodes for the comments
		// that appear above the functions (documentation)
		case *ast.FuncDecl:
			// gets the string from the funcDecl ->
			// Name (method in funcDecl struct) ->
			// Name (string inside of ident struct)
			s = x.Name.Name
			char := s[0]
			s2 := strings.ToUpper(s)
			if char == s2[0] {
				// check if function has comments/documentation, and
				// if it doesn't, add a comment in telling that it needs it
				if x.Doc == nil {
					r.Edits[r.filename(r.file)].Add(OffsetLength{r.program.Fset.Position(x.Pos()).Offset, 0}, "// TODO: FUNC NEEDS COMMENT INFO\n")
				}
			}
		// check all the gendecl nodes for the comments
		// that appear above structs and interfaces (documentation)
		case *ast.GenDecl:
			// get a list of the Spec type from gendecl
			aList := x.Specs
			// loop through the list to check each one
			for i, _ := range aList {
				// if it's a typespec, then it's a struct, interface,
				// or possibly a func, and since it's one of those,
				// check to see if it has comments
				if spec, ok := aList[i].(*ast.TypeSpec); ok {
					s = spec.Name.Name
					char := s[0]
					s2 := strings.ToUpper(s)
					if char == s2[0] {
						// check if the comment section of the
						// struct or interface is missing comments
						if x.Doc == nil {
							r.Edits[r.filename(r.file)].Add(OffsetLength{r.program.Fset.Position(x.Pos()).Offset, 0}, "// TODO: STRUCT/INTERFACE NEEDS COMMENT INFO\n")
						}
					}
				}
			}
		}
		return true
	})
}
