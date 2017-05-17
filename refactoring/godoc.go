// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE File.

// This File defines a refactoring that adds GoDoc comments to all exported
// top-level declarations in a File.

package refactoring

import (
	"go/ast"
	"go/token"

	"github.com/godoctor/godoctor/text"
)

// The AddGoDoc refactoring adds GoDoc comments to all exported top-level
// declarations in a File.
type AddGoDoc struct {
	base RefactoringBase
}

func (r *AddGoDoc) Description() *Description {
	return &Description{
		Name:      "Add GoDoc",
		Synopsis:  "Adds stub GoDoc comments where they are missing",
		Usage:     "",
		HTMLDoc:   godocDoc,
		Multifile: false,
		Params:    nil,
		Hidden:    false,
	}
}

func (r *AddGoDoc) Run(config *Config) *Result {
	r.base.Run(config)
	r.base.Log.ChangeInitialErrorsToWarnings()
	if r.base.Log.ContainsErrors() {
		return &r.base.Result
	}
	if !ValidateArgs(config, r.Description(), r.base.Log) {
		return &r.base.Result
	}

	r.removeSemicolons()
	r.addComments()
	r.base.FormatFileInEditor()
	return &r.base.Result
}

// removeSemicolons iterates through the top-level declarations in a File and
// the specs of general declarations, and if two consecutive declarations occur
// on the same line, splits them onto separate lines.  The intention is to
// split semicolon-separated declarations onto separate lines.
func (r *AddGoDoc) removeSemicolons() {
	for i, d := range r.base.File.Decls {
		if i > 0 {
			r.removeSemicolonBetween(r.base.File.Decls[i-1], r.base.File.Decls[i], "\n\n")
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
	line1 := r.base.Program.Fset.Position(node1.Pos()).Line
	line2 := r.base.Program.Fset.Position(node2.Pos()).Line
	if line1 == line2 {
		// Replace text between the end of the first declaration and
		// the start of the second declaration with the given
		// separators.  If there are comments, they will be eliminated,
		// but this should occur rarely enough we'll ignore it for now.
		offset := r.base.Program.Fset.Position(node1.End()).Offset
		length := r.base.Program.Fset.Position(node2.Pos()).Offset - offset
		r.base.Edits[r.base.Filename].Add(&text.Extent{offset, length}, replacement)
	}
}

// addComments inserts a comment immediately before all exported top-level
// declarations that do not already have an associated doc comment
func (r *AddGoDoc) addComments() {
	for _, d := range r.base.File.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl: // function or method declaration
			if ast.IsExported(decl.Name.Name) && decl.Doc == nil {
				r.addComment(decl, decl.Name.Name) //, 1)
			}
		case *ast.GenDecl: // type and value declarations
			// we want to try and be consistent with user commenting style,
			// so we want to detect if they're commenting individual specs for groups or not.
			switch decl.Tok {
			case token.CONST:
				fallthrough
			case token.VAR:
				if decl.Lparen.IsValid() && decl.Doc == nil {
					s := make(map[string]*ast.ValueSpec)
					addDeclComment := true
					for _, spec := range decl.Specs {
						spec := spec.(*ast.ValueSpec)
						name := spec.Names[0].Name
						if ast.IsExported(name) {
							if spec.Doc == nil {
								s[name] = spec
							} else {
								// they're commenting individual specs, we should too
								addDeclComment = false
							}
						}
					}
					if addDeclComment && len(s) > 0 {
						r.addComment(decl, "")
					} else {
						for name, spec := range s {
							r.addComment(spec, name)
						}
					}
				} else {
					spec := decl.Specs[0].(*ast.ValueSpec)
					if ast.IsExported(spec.Names[0].Name) && decl.Doc == nil {
						r.addComment(decl, spec.Names[0].Name)
					}
				}
			case token.TYPE:
				if decl.Lparen.IsValid() && decl.Doc == nil {
					s := make(map[string]*ast.TypeSpec)
					addDeclComment := true
					for _, spec := range decl.Specs {
						spec := spec.(*ast.TypeSpec)
						name := spec.Name.Name
						if ast.IsExported(name) {
							if spec.Doc == nil {
								s[name] = spec
							} else {
								// they're commenting individual specs, we should too
								addDeclComment = false
							}
						}
					}
					if addDeclComment && len(s) > 0 {
						r.addComment(decl, "")
					} else {
						for name, spec := range s {
							r.addComment(spec, name)
						}
					}
				} else {
					spec := decl.Specs[0].(*ast.TypeSpec)
					if ast.IsExported(spec.Name.Name) && decl.Doc == nil {
						r.addComment(decl, spec.Name.Name)
					}
				}
			default:
				continue
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
	insertOffset := r.base.Program.Fset.Position(decl.Pos()).Offset
	r.base.Edits[r.base.Filename].Add(&text.Extent{insertOffset, 0}, comment)
}

const godocDoc = `
  <h4>Purpose</h4>
  <p>This refactoring searches a file for exported declarations that do not have
  GoDoc comments and adds TODO comment stubs to those declarations.</p>
  <p>The refactored source code is formatted (similarly to gofmt).</p>

  <h4>Usage</h4>
  <p>This refactoring is applied to an entire file.  It does not require any
  particular text to be selected, and it does not prompt for any additional user
  input.</p>

  <h4>Example</h4>
  <p>In the following example, Exported, Shaper, and Rectangle are all exported
  but lack doc comments.  This refactoring adds a TODO comment for each.</p>
  <table cellspacing="5" cellpadding="15" style="border: 0;">
    <tr>
      <th>Before</th><th>&nbsp;</th><th>After</th>
    </tr>
    <tr>
      <td class="dotted">
  <pre>package main
import "fmt"

func main() {
    Exported()
}

func Exported() {
    fmt.Println("Hello, Go")
}

type Shaper interface {
}
 
type Rectangle struct {
}
  </pre>
      </td>
      <td>&nbsp;&nbsp;&nbsp;&nbsp;&rArr;&nbsp;&nbsp;&nbsp;&nbsp;</td>
      <td class="dotted">
      <pre>package main
import "fmt"

func main() {
    Exported()
}

<span class="highlight">// Exported TODO: NEEDS COMMENT INFO</span>
func Exported() {
    fmt.Println("Hello, Go")
}

<span class="highlight">// Exported TODO: NEEDS COMMENT INFO</span>
type Shaper interface {
}
  
<span class="highlight">// Exported TODO: NEEDS COMMENT INFO</span>
type Rectangle struct {
}
</pre>
      </td>
    </tr>
  </table>
`
