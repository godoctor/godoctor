package doctor

// This file defines the Rename refactoring.

import (
	//"fmt"
	"go/ast"
	//"strings"
	//"reflect"
	//"unicode"
	"code.google.com/p/go.tools/go/types"
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
	//	object *types.Object
	signature *types.Signature
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
		r.log.Log(FATAL_ERROR, "(Internal Error) Invalid arguments")
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
		if r.findIfFunction(ident) {
			if r.IsExportable(ident) {
				//fmt.Println("function is  exportable")
				r.findOccurrencesofFunction(ident)
			} else {
				//fmt.Println("function not exportable")
				r.findOccurrencesinPackage(ident)
			}
		} else if r.findIfMethod(ident) {
			//fmt.Println("selected method")
			if r.IsExportable(ident) {
				//TODO rename in all pkgs

			} else {
				//fmt.Println("method is not exportable")
				if r.isMethodinInterface(ident.Name) {
					r.getTypesWithMethod(ident.Name)
				} else {
					r.findOccurrencesinPackage(ident)
				}
			}

		} else {
			//fmt.Println("slected local variable")
			for _, occurrence := range r.findOccurrences(ident) {
				//TODO NOT HARD CODED FILENAME (reed)
				//iterate over files from a "fileSet"? importer? IDK my BFF Jill
				//r.editSet.Add(r.filename, occurrence, r.newName)
				r.editSet[r.filename].Add(occurrence, r.newName)
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
				offset := r.importer.Fset.Position(thisIdent.NamePos).Offset
				length := utf8.RuneCountInString(thisIdent.Name)
				result = append(result, OffsetLength{offset, length})
			}
		}

		return true
	})
	return result
}

//finds if selected identifier is name of a funciton

func (r *RenameRefactoring) findIfFunction(ident *ast.Ident) bool {

	var isafunction bool = false

	obj := r.pkgInfo.ObjectOf(ident)
	if obj == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
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
func (r *RenameRefactoring) findIfMethod(ident *ast.Ident) bool {

	var isamethod bool = false
	obj := r.pkgInfo.ObjectOf(ident)

	if obj == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
	}
	// fmt.Println("type of object is",types.Object.Type(obj))
	switch sig := types.Object.Type(obj).Underlying().(type) {
	case *types.Signature:
		recv := sig.Recv()
		if recv != nil {
			isamethod = true
			// fmt.Println("methodset", sig.MethodSet())
			//fmt.Println("Receivers type ", sig.Recv().Type())

		}
	default:

		// TODO error
	}

	return isamethod

}

//Finds if the given function name is exportable
//returns true if exportable and false otherwise

func (r *RenameRefactoring) IsExportable(ident *ast.Ident) bool {

	obj := r.pkgInfo.ObjectOf(ident)
	if obj == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
	}

	return types.Object.IsExported(obj)

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

			//fmt.Println("FILENAME:", r.importer.Fset.Position(file.Pos()).Filename)

			ast.Inspect(file, func(n ast.Node) bool {
				switch thisIdent := n.(type) {
				case *ast.Ident:
					if r.pkgInfo.ObjectOf(thisIdent) == decl {
						offset := r.importer.Fset.Position(thisIdent.NamePos).Offset
						length := utf8.RuneCountInString(thisIdent.Name)
						offsetlength := OffsetLength{offset, length}
						filename := r.importer.Fset.Position(file.Pos()).Filename

						//fmt.Println("filename ", filename, "edit", offsetlength, "newname", r.newName)
						//r.editSet.Add(filename, offsetlength, r.newName)
						if r.editSet[filename] == nil {
							r.editSet[filename] = NewEditSet()
						}
						r.editSet[filename].Add(offsetlength, r.newName)

					}

				}

				return true

			})

		}
	}
}

//finds and apply edits to all occurances in the same pkg

func (r *RenameRefactoring) findOccurrencesinPackage(ident *ast.Ident) {

	decl := r.pkgInfo.ObjectOf(ident)
	if decl == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
	}

	//fmt.Println("package name is ", r.pkgInfo.Pkg.Name())

	for _, file := range r.pkgInfo.Files {

		//fmt.Println("FILENAME:", r.importer.Fset.Position(file.Pos()).Filename)

		ast.Inspect(file, func(n ast.Node) bool {
			switch thisIdent := n.(type) {
			case *ast.Ident:
				if r.pkgInfo.ObjectOf(thisIdent) == decl {
					offset := r.importer.Fset.Position(thisIdent.NamePos).Offset
					length := utf8.RuneCountInString(thisIdent.Name)
					offsetlength := OffsetLength{offset, length}
					filename := r.importer.Fset.Position(file.Pos()).Filename
					//r.editSet.Add(filename, offsetlength, r.newName)
					r.editSet[filename].Add(offsetlength, r.newName)
					//fmt.Println("filename ", filename, "edit", offsetlength, "newname", r.newName)

				}

			}

			return true

		})

	}

}

//finds if selected method lies in any of the interfaces in pkg
func (r *RenameRefactoring) isMethodinInterface(identname string) bool {

	var isinInterface = false
	methodname := identname
	//fmt.Println("package name is ",r.pkgInfo.Pkg.Name())

	for _, file := range r.pkgInfo.Files {

		//fmt.Println("FILENAME:", r.importer.Fset.Position(file.Pos()).Filename)

		ast.Inspect(file, func(n ast.Node) bool {
			switch thisIdent := n.(type) {
			case *ast.InterfaceType:
				for _, name := range thisIdent.Methods.List[0].Names {
					if name.Name == methodname {
						isinInterface = true
						//fmt.Println("method name is present in interface")
						//fmt.Println("total names",thisIdent.Methods.List[0].Names)
						//break
					}
				}

			}

			return true

		})

	}

	return isinInterface
}

//finds all interface and sturct types which has the selected method

func (r *RenameRefactoring) getTypesWithMethod(methodname string) {

	var TypesofInterfaces []*types.Interface
	var TypesofStructs []types.Type
	var OtherTypes []types.Type
	//fmt.Println("inside getTypes")

	for _, file := range r.pkgInfo.Files {

		ast.Inspect(file, func(n ast.Node) bool {

			switch thisSpec := n.(type) {

			case *ast.TypeSpec:
				//TODO Use ast.Ident , to include pointers
				// Gives Run time error now??
				obj := r.pkgInfo.ObjectOf(thisSpec.Name)

				//switch sig := types.Object.Type(obj).Underlying().(type) {
				switch sig := r.pkgInfo.TypeOf(thisSpec.Name).Underlying().(type) {

				case *types.Interface:
					if sig.MethodSet().Lookup(r.pkgInfo.Pkg, methodname) != nil {
						TypesofInterfaces = append(TypesofInterfaces, sig)
					}

				case *types.Struct:
					if types.Object.Type(obj).MethodSet().Lookup(r.pkgInfo.Pkg, methodname) != nil {

						//fmt.Println("methodset of struct",types.Object.Type(obj).MethodSet())
						TypesofStructs = append(TypesofStructs, types.Object.Type(obj))
						//fmt.Println("struct types",types.Object.Type(obj))
					}

				case *types.Pointer:
					//TODO get types of pointer types
					//fmt.Println("methodset of pointer", sig.Elem().MethodSet())
				default:

				}

			case *ast.FuncDecl:
				if thisSpec.Name.Name == methodname {
					//types := thisSpec.Recv.List[0].Type
					//types    := r.pkgInfo.TypeOf(thisSpec.Name).Underlying()
					//types  := thisSpec.Name.Obj.Type
					//obj    := r.pkgInfo.ObjectOf(thisSpec.Name)
					//types  := types.Object.Type(obj)
					obj := r.pkgInfo.ObjectOf(thisSpec.Recv.List[0].Names[0])
					types := types.Object.Type(obj)

					OtherTypes = append(OtherTypes, types)

				}

			}

			return true

		})

	}
	//fmt.Println("type of struct",TypesofStructs[0])
	//fmt.Println("methoset of struct",TypesofStructs[0].MethodSet())

	//fmt.Println("type of Interface", TypesofInterfaces[0])
	//fmt.Println("methoset of Interface", TypesofInterfaces[0].MethodSet())

	//fmt.Println("other types", OtherTypes)
	//fmt.Println("does struct implement interface", types.Implements(OtherTypes[0], TypesofInterfaces[0], true))

}
