package doctor

// This file defines the Rename refactoring.

import (
	"go/ast"
	"unicode/utf8"
)

// A RenameRefactoring is used to rename identifiers in Go programs.
// It implements the Refactoring interface.
//
// To rename an identifier:
// * Create a RenameRefactoring.
// * Invoke SetSelection to determine what identifier to rename.
// * Invoke SetNewName to set the new name for the identifier.
// * Invoke Run to construct the EditSet.
// * Invoke GetResult to get the resulting Log and EditSet.
//
type RenameRefactoring struct {
	RefactoringBase
	newName string
}

func (r *RenameRefactoring) Name() string {
	return "Rename"
}

func (r *RenameRefactoring) SetNewName(newName string) {
	r.newName = newName
}

func (r *RenameRefactoring) Run() {
	if r.selectedNode == nil {
		r.log.Log(FATAL_ERROR, "selection cannot be null")
		return // SetSelection did not succeed
	}

	if r.newName == "" {
		r.log.Log(FATAL_ERROR, "newName cannot be empty")
		return
	}

	switch ident := r.selectedNode.(type) {
	case *ast.Ident:
		for _, occurrence := range r.findOccurrences(ident) {
			r.editSet.Add(occurrence, r.newName)
		}
		//fmt.Println(editSet.String())
		return

	default:
		r.log.Log(FATAL_ERROR, "Please select an identifier")
		return
	}
}

// Finds all of the references in an AST to a single declaration
func (r *RenameRefactoring) findOccurrences(ident *ast.Ident) []OffsetLength {
	decl := r.pkgInfo.ObjectOf(ident)
	//fmt.Println("Declaration is ", decl)

	result := make([]OffsetLength, 0, 0)
	ast.Inspect(r.file, func(n ast.Node) bool {
		switch thisIdent := n.(type) {
		case *ast.Ident:
			if r.pkgInfo.ObjectOf(thisIdent) == decl {
				offset := r.fset.Position(thisIdent.NamePos).Offset
				length := utf8.RuneCountInString(thisIdent.Name)
				result = append(result, OffsetLength{offset, length})
			}
		}
		return true
	})
	return result
}
