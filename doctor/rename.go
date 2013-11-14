package doctor

// This file defines the Rename refactoring.

import (
	"fmt"
	"go/ast"
	"strings"
	"unicode"
	"unicode/utf8"
	//"reflect"
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

func (r *RenameRefactoring) Configure(args []string) bool {
	if len(args) == 1 {
		r.SetNewName(args[0])
		return true
	} else {
		r.log.Log(FATAL_ERROR, "Marker is missing new name")
		return false
	}
}

func (r *RenameRefactoring) GetParams() []string {
	return []string{"New Name"}
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
		//fmt.Println(editSet.String())

		identname := ident.Name

		if r.findIfFunctionName(identname) {

			if r.IsFunctionExportable(identname) {

				//TODO make a call to find if method
				//r.FindIfMethod(identname)

				//fmt.Println("function is  exportable")	
				r.findOccurrencesofFunction(ident)

			} else {

				// fmt.Println("function is not exportable")
				//for now treat it same as local variable 

				for _, occurrence := range r.findOccurrences(ident) {
					//TODO add edits to all files in same pkg
					r.editSet.Add(r.filename, occurrence, r.newName)
				}

			}

		} else {

			for _, occurrence := range r.findOccurrences(ident) {
				//TODO NOT HARD CODED FILENAME (reed)
				//iterate over files from a "fileSet"? importer? IDK my BFF Jill
				r.editSet.Add(r.filename, occurrence, r.newName)
				// fmt.Println("this is the  selected filename",r.filename)
			}

		}

		//r.checkForErrors()

		return

	default:
		r.log.Log(FATAL_ERROR, "Please select an identifier")
		return
	}
}

// Finds all of the references in an AST to a single declaration
func (r *RenameRefactoring) findOccurrences(ident *ast.Ident) []OffsetLength {
	decl := r.pkgInfo.ObjectOf(ident)
	if decl == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
		return []OffsetLength{}
	}

	//result := make([]OffsetLength, 0, 0)
	var result []OffsetLength
	ast.Inspect(r.file, func(n ast.Node) bool {
		switch thisIdent := n.(type) {
		case *ast.Ident:
			if r.pkgInfo.ObjectOf(thisIdent) == decl {
				offset := r.importer.Fset.Position(thisIdent.NamePos).Offset
				length := utf8.RuneCountInString(thisIdent.Name)
				result = append(result, OffsetLength{offset, length})
			}
		}

		return true
	})
	return result
}

//TODO:reddy make search more efficient
//find if selected identifier is name of a funciton

func (r *RenameRefactoring) findIfFunctionName(identname string) bool {

	var isafunction bool = false

	for _, pkgInfom := range r.importer.AllPackages() {

		for _, file := range pkgInfom.Files {

			ast.Inspect(file, func(n ast.Node) bool {
				switch thisIdent := n.(type) {
				case *ast.FuncDecl:

					if thisIdent.Name.Name == identname {

						fmt.Println("selected identifier is  name of a function")

						isafunction = true

					}

				}
				return true
			})

		}
	}
	return isafunction

}

//Finds if the given function name is exportable
//returns true if exportable and false otherwise

func (r *RenameRefactoring) IsFunctionExportable(funcname string) bool {

	splitname := make([]string, 2)
	splitname = strings.SplitAfterN(funcname, "", 2)

	runeofstring, _ := utf8.DecodeRuneInString(splitname[0])
	//fmt.Println(reflect.TypeOf(runeofstring))
	// fmt.Println(unicode.IsUpper(runeofstring))
	return unicode.IsUpper(runeofstring)

}

//TODO Reddy: does not work as intended with pakcages
//     importer is loaded with duplicates Need to verify after avoiding that       

//Finds the references of function name from all files 
//Adds all the references to editSet

func (r *RenameRefactoring) findOccurrencesofFunction(ident *ast.Ident) {
	decl := r.pkgInfo.ObjectOf(ident)
	if decl == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
	}

	for _, pkgInfom := range r.importer.AllPackages() {

		r.pkgInfo = pkgInfom

		for _, file := range r.pkgInfo.Files {

			fmt.Println("FILENAME:", r.importer.Fset.Position(file.Pos()).Filename)

			ast.Inspect(file, func(n ast.Node) bool {
				switch thisIdent := n.(type) {
				case *ast.Ident:
					if r.pkgInfo.ObjectOf(thisIdent) == decl {
						offset := r.importer.Fset.Position(thisIdent.NamePos).Offset
						length := utf8.RuneCountInString(thisIdent.Name)
						offsetlength := OffsetLength{offset, length}
						filename := r.importer.Fset.Position(file.Pos()).Filename
						r.editSet.Add(filename, offsetlength, r.newName)
						fmt.Println("filename ", filename, "edit", offsetlength, "newname", r.newName)
						//fmt.Println("filename where edits are applied",filename)					
					}

				}

				return true

			})

		}
	}
}
