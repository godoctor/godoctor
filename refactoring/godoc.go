// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a refactoring that adds GoDoc comments to all exported
// top-level declarations in a file.

package refactoring

import (
	"go/ast"

	"golang-refactoring.org/go-doctor/text"
)

// The AddGoDoc refactoring adds GoDoc comments to all exported top-level
// declarations in a file.
type AddGoDoc struct {
	refactoringBase
}

func (r *AddGoDoc) Description() *Description {
	return &Description{
		Name:    "Add GoDoc",
		Params:  nil,
		Quality: Development,
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

	r.removeSemicolonsBetweenDecls()
	r.addComments()
	return &r.Result
}

// removeSemicolonsBetweenDecls iterates through the top-level declarations in
// a file, and if two consecutive declarations occur on the same line, splits
// them onto separate lines.
func (r *AddGoDoc) removeSemicolonsBetweenDecls() {
	for i := 0; i < len(r.file.Decls)-1; i++ {
		// check if the 2 declarations are on the same line
		line1 := r.program.Fset.Position(r.file.Decls[i].Pos()).Line
		line2 := r.program.Fset.Position(r.file.Decls[i+1].Pos()).Line
		if line1 == line2 {
			// replace separator (;) with 2 newlines
			offset1 := r.program.Fset.Position(r.file.Decls[i].End()).Offset
			offset2 := r.program.Fset.Position(r.file.Decls[i+1].Pos()).Offset - offset1
			r.Edits[r.filename(r.file)].Add(text.Extent{offset1, offset2}, "\n\n")
		}
	}
}

// addComments inserts a comment immediately before all exported top-level
// declarations that do not already have an associated doc comment
func (r *AddGoDoc) addComments() {
	for _, d := range r.file.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl: // function or method declaration
			fcomment := "// " + decl.Name.Name + " TODO: FUNC NEEDS COMMENT INFO\n"
			if ast.IsExported(decl.Name.Name) && decl.Doc == nil {
				r.addComment(decl, fcomment)
			}
		case *ast.GenDecl: // types (including structs/interfaces)
			for _, spec := range decl.Specs {
				if spec, ok := spec.(*ast.TypeSpec); ok {
					sIcomment := "// " + spec.Name.Name + " TODO: STRUCT/INTERFACE NEEDS COMMENT INFO\n"
					if ast.IsExported(spec.Name.Name) && spec.Doc == nil {
						if decl.Lparen.IsValid() {
							r.addComment(spec, sIcomment)
						} else {
							r.addComment(decl, sIcomment)
						}
					}
				}
			}
		}
	}
}

// addComment inserts the given comment string immediately before the given
// declaration
func (r *AddGoDoc) addComment(decl ast.Node, comment string) {
	insertOffset := r.program.Fset.Position(decl.Pos()).Offset
	r.Edits[r.filename(r.file)].Add(text.Extent{insertOffset, 0}, comment)
}
