// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"
	"go/token"
	"strings"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
	"golang-refactoring.org/go-doctor/text"
)

/* -=-=- Search by Identifier  -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// TODO(review D7): I think I mentioned this before but this function has a
// strange signature: it mixes objects from two non-adjacent layers of the
// design abstraction: semantic objects (e.g. types.Object) and concrete syntax
// (text.OffsetLength). In between these two layers is that of abstract syntax (e.g.
// ast.Ident). This suggests that the function is doing too much; perhaps it
// should just be returning a set of *ast.Idents for a later function to map
// down to concrete syntax.

// FindOccurrences receives an identifier and returns the set of all
// identically named identifiers that refer to the same object as that
// identifier.
func FindOccurrences(obj types.Object, prog *loader.Program) map[*ast.Ident]bool {
	decls := map[types.Object]bool{obj: true}
	if isMethod(obj) {
		decls = FindDeclarationsAcrossInterfaces(obj, prog)
	}

	result := make(map[*ast.Ident]bool)
	for pkgInfo := range packages(decls, prog) {
		for id, obj := range pkgInfo.Defs {
			if decls[obj] {
				result[id] = true
			}
		}
		for id, obj := range pkgInfo.Uses {
			if decls[obj] {
				result[id] = true
			}
		}
	}
	return result
}

// packages returns a set of PackageInfos that may reference the given
// Objects.  If at least one of the given declarations is exported, the method
// returns all the packages of this program; otherwise, it returns the
// package(s) containing the given declarations.
func packages(decls map[types.Object]bool, program *loader.Program) map[*loader.PackageInfo]bool {
	// XXX(review D7): If performance is a concern, you could return only
	// the packages in the reverse transitive closure of the package import
	// graph, rather than all the packages.

	result := make(map[*loader.PackageInfo]bool)
	for decl := range decls {
		if decl.Exported() {
			return allPackages(program)
		}
		pkgInfo := program.AllPackages[decl.Pkg()]
		result[pkgInfo] = true
	}
	return result
}

type finder struct {
	program *loader.Program
}

//TODO : Make the search robust for packagenames in importspec
func FindReferencesToPackage(pkgName string, program *loader.Program) map[string][]text.Extent {
	return (&finder{program}).findReferencesToPackage(pkgName)
}

func (r *finder) findReferencesToPackage(pkgName string) map[string][]text.Extent {
	result := make(map[string][]text.Extent)
	for pkgInfo := range allPackages(r.program) {
		for id, obj := range pkgInfo.Defs {
			if obj == nil && id.Name == pkgName {

				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}
		for id, obj := range pkgInfo.Uses {
			if (obj == nil || obj.Name() == pkgName) && id.Name == pkgName {

				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}

		for node, pkgObject := range pkgInfo.Implicits {
			if pkgObject.Name() == pkgName {

				filename := r.positionofObject(pkgObject).Filename

				result[filename] = append(result[filename],
					r.offsetLengthofObject(node, pkgObject))
			}

			for _, file := range pkgInfo.Files {

				ast.Inspect(file, func(node ast.Node) bool {
					switch n := node.(type) {
					case *ast.ImportSpec:
						if n.Name != nil && strings.Replace(n.Path.Value, "\"", "", 2) == pkgName {
							//fmt.Println("pkg name with local rename")
							filename := r.positionofPkg(n.Path).Filename

							result[filename] = append(result[filename],
								r.offsetLengthofPkg(n.Path))

						}

					}
					return true
				})
			}

		}

	}

	return result
}

func FindOccurrencesOfCaseVar(identName string, program *loader.Program) map[string][]text.Extent {
	return (&finder{program}).occurrencesofCaseVar(identName)
}

//TODO : Make the search robust
func (r *finder) occurrencesofCaseVar(identName string) map[string][]text.Extent {

	result := make(map[string][]text.Extent)
	for pkgInfo := range allPackages(r.program) {

		for id, obj := range pkgInfo.Uses {
			if (obj == nil || obj.Name() == identName) && id.Name == identName {
				//fmt.Println("slected  case var and types.var is",obj.(*types.Var))
				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}

		for node, pkgObject := range pkgInfo.Implicits {

			if pkgObject.Name() == identName {

				//fmt.Println("slected  case var and types.var is",obj.(*types.Var))
				filename := r.positionofObject(pkgObject).Filename

				result[filename] = append(result[filename],
					r.offsetLengthofObject(node, pkgObject))
			}

		}

	}

	return result
}

func (r *finder) position(id *ast.Ident) token.Position {
	return r.program.Fset.Position(id.NamePos)
}
func (r *finder) positionofObject(pkgObject types.Object) token.Position {
	return r.program.Fset.Position(pkgObject.Pos())
}
func (r *finder) positionofPkg(id *ast.BasicLit) token.Position {
	return r.program.Fset.Position(id.ValuePos)
}

func (r *finder) offsetLength(id *ast.Ident) text.Extent {
	position := r.position(id)
	offset := position.Offset
	length := len(id.Name)
	return text.Extent{offset, length}
}

func (r *finder) offsetLengthofObject(node ast.Node, obj types.Object) text.Extent {

	var offset int
	position := r.positionofObject(obj)
	offset = position.Offset + 1
	length := len(obj.Name())

	switch ident := node.(type) {
	case *ast.ImportSpec:

		if strings.Replace(ident.Path.Value, "\"", "", 2) != obj.Name() {

			offset = position.Offset + len(strings.Replace(ident.Path.Value, "\"", "", 2)) - len(obj.Name()) + 1
		}

	case *ast.CaseClause:

		offset = offset - 1
	}

	return text.Extent{offset, length}
}

func (r *finder) offsetLengthofPkg(id *ast.BasicLit) text.Extent {

	var offset int
	position := r.positionofPkg(id)
	offset = position.Offset + 1
	length := len(id.Value) - 2

	//fmt.Println("offset , length ", offset, length)

	/*switch ident := node.(type) {
	case *ast.ImportSpec:

		if strings.Replace(ident.Path.Value, "\"", "", 2) != obj.Name() {

			offset = position.Offset + len(strings.Replace(ident.Path.Value, "\"", "", 2)) - len(obj.Name()) + 1
		}
	}*/

	return text.Extent{offset, length}
}

func allPackages(prog *loader.Program) map[*loader.PackageInfo]bool {
	result := map[*loader.PackageInfo]bool{}
	for _, pkgInfo := range prog.AllPackages {
		result[pkgInfo] = true
	}
	return result
}
