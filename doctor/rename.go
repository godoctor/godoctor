// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

// This file defines a refactoring to rename variables, functions, methods, structs, and interfaces.
// (TODO: It cannot yet rename packages.)

import (
	"go/ast"

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
	r.newName = newName
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

	// TODO: Check if r.newName is a valid Go identifier

	switch ident := r.selectedNode.(type) {
	case *ast.Ident:
		r.rename(ident)

	default:
		r.log.Log(FATAL_ERROR, "Please select an identifier to rename.")
		return
	}
}

func (r *RenameRefactoring) rename(ident *ast.Ident) {

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

//TODO pkgs not identified
/*else if r.findIfPackage(ident) {
          	if r.IsExportable(ident) {

                  fmt.Println("package is exportable")
                  allOccurrences = r. findOccurrences(true,ident)
                   r.addOccurrences(allOccurrences)
               } else {
                fmt.Println("package is not exportable")
                  allOccurrences = r. findOccurrences(false,ident)
                   r.addOccurrences(allOccurrences)
                  }

	    }
*/

/*
// Finds all of the references to a single declaration in one AST
// (unlike findOccurrences, which searches the entire package)
func (r *RenameRefactoring) findOccurrencesofVar(ident *ast.Ident) []OffsetLength {

	var result []OffsetLength

	decl := r.pkgInfo.ObjectOf(ident)
	if decl == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
		return []OffsetLength{}
	}

	ast.Inspect(r.file, func(n ast.Node) bool {
		switch thisIdent := n.(type) {
		case *ast.Ident:
			if r.pkgInfo.ObjectOf(thisIdent) == decl {
				offset := r.program.Fset.Position(thisIdent.NamePos).Offset
				length := utf8.RuneCountInString(thisIdent.Name)
				result = append(result, OffsetLength{offset, length})
			}
		}

		return true
	})
	return result
}
*/

/*
//finds if selected identifier is name of a funciton
func (r *RenameRefactoring) findIfFunction(ident *ast.Ident) bool {
	var isafunction bool = false

	obj := r.pkgInfo.ObjectOf(ident)
	if obj == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
		return false
	}

	switch sig := types.Object.Type(obj).(type) {
	case *types.Signature:
		recv := sig.Recv()
		if recv == nil {
			isafunction = true

		}

	default:
		// TODO error
	}

	return isafunction
}
*/

//addOccurrences Adds all the Occurences to the editset
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
