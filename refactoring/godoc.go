// Copyright 2014 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a refactoring that adds GoDoc comments to all exported
// top-level declarations in a file.

package refactoring

import (
	"go/ast"

	"github.com/godoctor/godoctor/text"
)

// The AddGoDoc refactoring adds GoDoc comments to all exported top-level
// declarations in a file.
type AddGoDoc struct {
	refactoringBase
}

func (r *AddGoDoc) Description() *Description {
	return &Description{
		Name:      "Add GoDoc",
		Synopsis:  "Adds stub GoDoc comments where they are missing",
		Usage:     "",
		Multifile: false,
		Params:    nil,
		Hidden:    false,
	}
}

func (r *AddGoDoc) Run(config *Config) *Result {
	r.refactoringBase.Run(config)
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}
	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	r.removeSemicolons()
	r.addComments()
	r.formatFileInEditor()
	return &r.Result
}

// removeSemicolons iterates through the top-level declarations in a file and
// the specs of general declarations, and if two consecutive declarations occur
// on the same line, splits them onto separate lines.  The intention is to
// split semicolon-separated declarations onto separate lines.
func (r *AddGoDoc) removeSemicolons() {
	for i, d := range r.file.Decls {
		if i > 0 {
			r.removeSemicolonBetween(r.file.Decls[i-1], r.file.Decls[i], "\n\n")
		}
		if decl, ok := d.(*ast.GenDecl); ok {
			for j, spec := range decl.Specs {
				if spec, ok := spec.(*ast.TypeSpec); ok {
					if ast.IsExported(spec.Name.Name) && spec.Doc == nil && j > 0 {
						r.removeSemicolonBetween(decl.Specs[j-1], decl.Specs[j], "\n")
					}
				}
			}
		}

	}
}

func (r *AddGoDoc) removeSemicolonBetween(node1, node2 ast.Node, replacement string) {
	// Check if the 2 declarations are on the same line
	line1 := r.program.Fset.Position(node1.Pos()).Line
	line2 := r.program.Fset.Position(node2.Pos()).Line
	if line1 == line2 {
		// Replace text between the end of the first declaration and
		// the start of the second declaration with the given
		// separators.  If there are comments, they will be eliminated,
		// but this should occur rarely enough we'll ignore it for now.
		offset := r.program.Fset.Position(node1.End()).Offset
		length := r.program.Fset.Position(node2.Pos()).Offset - offset
		r.Edits[r.filename].Add(&text.Extent{offset, length}, replacement)
	}
}

// addComments inserts a comment immediately before all exported top-level
// declarations that do not already have an associated doc comment
func (r *AddGoDoc) addComments() {
	for _, d := range r.file.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl: // function or method declaration
			if ast.IsExported(decl.Name.Name) && decl.Doc == nil {
				r.addComment(decl, decl.Name.Name) //, 1)
			}
		case *ast.GenDecl: // types (including structs/interfaces)
			for _, spec := range decl.Specs {
				if spec, ok := spec.(*ast.TypeSpec); ok {
					if ast.IsExported(spec.Name.Name) && spec.Doc == nil {
						if decl.Lparen.IsValid() {
							r.addComment(spec, spec.Name.Name) //, 2)
						} else {
							r.addComment(decl, spec.Name.Name) //, 1)
						}
					}
				}
			}
		}
	}
}

// addComment inserts the given comment string immediately before the given
// declaration
func (r *AddGoDoc) addComment(decl ast.Node, comment string) { //, count int) {
	//if count == 1 {
	comment = "// " + comment + " TODO: NEEDS COMMENT INFO\n"
	//} else if count == 2 {
	//	comment = "\n// " + comment + " TODO: NEEDS COMMENT INFO\n"
	//}
	insertOffset := r.program.Fset.Position(decl.Pos()).Offset
	r.Edits[r.filename].Add(&text.Extent{insertOffset, 0}, comment)
}
