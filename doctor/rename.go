package doctor

// This file defines a refactoring to rename variables, functions, methods, structs, and interfaces.
// (TODO: It cannot yet rename packages.)

import (
	"code.google.com/p/go.tools/go/types"
	"fmt"
	"go/ast"
	"unicode/utf8"
)

// A RenameRefactoring is used to rename identifiers in Go programs.
// It implements the Refactoring interface.
// // To rename an identifier:
// * Create a RenameRefactoring.
// * Invoke SetSelection to determine what identifier to rename.
// * Invoke SetNewName to set the new name for the identifier.
// * Invoke Run to construct the EditSet.
// * Invoke GetResult to get the resulting Log and EditSet.
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
		r.log.Log(FATAL_ERROR, "selection cannot be null")
		return // SetSelection did not succeed
	}

	if r.newName == "" {
		r.log.Log(FATAL_ERROR, "newName cannot be empty")
		return
	}

	// TODO: Check if r.newName is a valid Go identifier

	allOccurrences := make(map[string][]OffsetLength)
	switch ident := r.selectedNode.(type) {

	case *ast.Ident:
		if r.isMethod(ident) {
			if r.isExportable(ident) {
				allOccurrences = r.findOccurrencesIncludingClosure(r.isMethodinInterface(true, ident.Name), ident)
			} else {
				if r.isMethodinInterface(false, ident.Name) {
					allOccurrences = r.findOccurrencesIncludingClosure(false, ident)
				} else {
					allOccurrences = r.findOccurrences(false, ident)
				}
			}
		} else {
			allOccurrences = r.findOccurrences(r.isExportable(ident), ident)
		}

		r.addOccurrences(allOccurrences)
		//TODO: r.checkForErrors()

		return

	default:
		r.log.Log(FATAL_ERROR, "Please select an identifier")
		return
	}
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
				offset := r.importer.Fset.Position(thisIdent.NamePos).Offset
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

// isMethod returns true if the given identifier is the name of a method
func (r *RenameRefactoring) isMethod(ident *ast.Ident) bool {
	obj := r.pkgInfo.ObjectOf(ident)
	if obj == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
		return false
	}

	switch sig := types.Object.Type(obj).Underlying().(type) {
	case *types.Signature:
		return sig.Recv() != nil

	default:
		return false
	}
}

//finds if selected identifier is name of a package
/*func (r *RenameRefactoring) findIfPackage(ident *ast.Ident) bool  {

	var isapackage bool = false

	obj := r.pkgInfo.ObjectOf(ident)
	if obj == nil {
			r.log.Log(FATAL_ERROR, "Unable to find declaration")
             return false
	}
         fmt.Println("type",types.Object.Type(obj).Underlying())
	/*switch pkg := types.Object.Type(obj).Underlying().(type) {
	case *types.Package:
         	//pkgname := pkg.Name()
       		//if pkgname == ident.Name {
         		isapackage = true  

		//}          

	default:

		fmt.Println(pkg)
		// TODO error
	}

	return isapackage
}*/

func (r *RenameRefactoring) isExportable(ident *ast.Ident) bool {
	obj := r.pkgInfo.ObjectOf(ident)
	if obj == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
		return false
	}

	return types.Object.IsExported(obj)
}

// isMethodinInterface returns true if the given method is in the method set of
// any interface.  If the all parameter is true, it searches all packages loaded
// into the importer; if it is false, it searches only the current package.
func (r *RenameRefactoring) isMethodinInterface(all bool, methodname string) bool {
	isinInterface := false
	for _, pkgInfo := range r.getPackages(all) {
		r.pkgInfo = pkgInfo // TODO: Why is this necessary?
		for _, file := range r.pkgInfo.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				switch thisIdent := n.(type) {
				case *ast.InterfaceType:
					if len(thisIdent.Methods.List) != 0 {
						for _, name := range thisIdent.Methods.List[0].Names {
							if name.Name == methodname {
								isinInterface = true
							}
						}
					}
				}
				return true
			})
		}
	}
	return isinInterface
}

// getTypesWithMethod returns all of the types that implement the given method 
func (r *RenameRefactoring) getTypesWithMethod(all bool, method *ast.Ident) []types.Type {

	methodobj := r.pkgInfo.ObjectOf(method)
	methodname := method.Name

	var TypesofInterfaces []*types.Interface
	var OtherTypes []types.Type
	var affected []types.Type
	var typ types.Type

	for _, pkgInfo := range r.getPackages(all) {
		r.pkgInfo = pkgInfo // TODO: Why is this necessary?
		for _, file := range r.pkgInfo.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				switch thisSpec := n.(type) {
				case *ast.TypeSpec:
					switch sig := r.pkgInfo.TypeOf(thisSpec.Name).Underlying().(type) {
					case *types.Interface:
						if sig.MethodSet().Lookup(r.pkgInfo.Pkg, methodname) != nil {
							TypesofInterfaces = append(TypesofInterfaces, sig)
						}
					}

				case *ast.FuncDecl:

					if thisSpec.Name.Name == methodname && r.CheckforsameParam(r.pkgInfo.ObjectOf(thisSpec.Name), methodobj) {

						obj := r.pkgInfo.ObjectOf(thisSpec.Recv.List[0].Names[0])
						types := types.Object.Type(obj)
						OtherTypes = append(OtherTypes, types)

					}

				}

				return true
			})
		}
	}

	var closure map[types.Type][]types.Type = closure(TypesofInterfaces, OtherTypes)
	for typ, affected = range closure {
		fmt.Printf("Renaming %s will also affect %v\n", typ, affected)
		fmt.Println("methodsets of affectedtype", affected[0].MethodSet())
	}
	return affected
	// TODO: Can we just return closure[something]?
}

//findOccurrencesinAffectedTypes finds all the Occurrences of Methods in affected types;if all parameter is true
// it searches for Occurrences in all packages loaded by the importer ; if it is false it searches only the current package 
func (r *RenameRefactoring) findOccurrencesinAffectedTypes(all bool, affected []types.Type, methodname string) map[string][]OffsetLength {

	result := make(map[string][]OffsetLength)

	pkgs := r.getPackages(all)

	for _, affectedtype := range affected {

		for _, pkgInfo := range pkgs {

			r.pkgInfo = pkgInfo

			for _, file := range r.pkgInfo.Files {

				ast.Inspect(file, func(n ast.Node) bool {
					switch thisIdent := n.(type) {
					case *ast.TypeSpec:

						if r.pkgInfo.TypeOf(thisIdent.Name).Underlying() == affectedtype {

							switch typeofthisIdent := thisIdent.Type.(type) {

							case *ast.InterfaceType:

								for _, MethodList := range typeofthisIdent.Methods.List {

									if MethodList.Names[0].Name == methodname {
										offset := r.importer.Fset.Position(MethodList.Names[0].NamePos).Offset
										length := utf8.RuneCountInString(MethodList.Names[0].Name)

										filename := r.importer.Fset.Position(file.Pos()).Filename

										result[filename] = append(result[filename], OffsetLength{offset, length})

									}
								}

							}
						}

					case *ast.FuncDecl:

						if thisIdent.Name.Name == methodname {

							obj := r.pkgInfo.ObjectOf(thisIdent.Recv.List[0].Names[0])
							methodtype := types.Object.Type(obj)

							if methodtype == affectedtype {

								offset := r.importer.Fset.Position(thisIdent.Name.NamePos).Offset
								length := utf8.RuneCountInString(thisIdent.Name.Name)
								filename := r.importer.Fset.Position(file.Pos()).Filename
								result[filename] = append(result[filename], OffsetLength{offset, length})

							}

						}
					}

					return true

				})

			}

		}
	}

	return result
}

func (r *RenameRefactoring) findOccurrencesIncludingClosure(all bool, ident *ast.Ident) map[string][]OffsetLength {

	allOccurrences := make(map[string][]OffsetLength)
	OccurrencesofMethods := r.findOccurrences(all, ident)

	affected := r.getTypesWithMethod(all, ident)

	OccurrencesinTypes := r.findOccurrencesinAffectedTypes(all, affected, ident.Name)

	for filename, occurrences := range OccurrencesofMethods {
		for _, occurrence := range occurrences {

			allOccurrences[filename] = append(allOccurrences[filename], occurrence)

		}
	}

	for filename, occurrences := range OccurrencesinTypes {
		for _, occurrence := range occurrences {

			var isDuplicate bool
			//eliminate duplicate occurrences
			isDuplicate = isOccurrenceDuplicate(allOccurrences, filename, occurrence)
			if !isDuplicate {
				allOccurrences[filename] = append(allOccurrences[filename], occurrence)
			}

		}
	}

	return allOccurrences
}

//addOccurrences Adds all the Occurences to the editset
func (r *RenameRefactoring) addOccurrences(allOccurrences map[string][]OffsetLength) {

	for filename, occurrences := range allOccurrences {
		for _, occurrence := range occurrences {
			if r.editSet[filename] == nil {
				r.editSet[filename] = NewEditSet()
			}

			r.editSet[filename].Add(occurrence, r.newName)

			//fmt.Println("filename", filename, "occurance", occurrence)

		}

	}

}

func isOccurrenceDuplicate(allOccurrences map[string][]OffsetLength, filename string, occurrence OffsetLength) bool {

	for _, occurrenceinfile := range allOccurrences[filename] {

		if occurrenceinfile == occurrence {

			return true
		}

	}

	return false

}

//CheckforSameParam checks if given  objects (objects of method)  have same parameters;
//returns true if they have same paramets ;returns false otherwise
//func (r *RenameRefactoring) CheckforsameParam(ident *ast.Ident, method *ast.Ident) bool {
func (r *RenameRefactoring) CheckforsameParam(identObj types.Object, methodObj types.Object) bool {

	var identParams *types.Tuple
	var methodParams *types.Tuple

	if identObj == nil || methodObj == nil {
		r.log.Log(FATAL_ERROR, "Unable to find declaration")
		return false

	}

	switch sig := types.Object.Type(identObj).Underlying().(type) {
	case *types.Signature:
		identParams = sig.Params()

	default:
		return false
	}

	switch sig := types.Object.Type(methodObj).Underlying().(type) {
	case *types.Signature:
		methodParams = sig.Params()

	default:
		return false
	}

	if identParams == methodParams {

		return true

	}

	return false
}
