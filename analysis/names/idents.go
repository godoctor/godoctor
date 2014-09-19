// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"fmt"
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

// FindOccurrences finds the location of all identifiers that are direct or
// indirect references to the same object as given identifier.  The returned
// map maps filenames to a slice of (offset, length) pairs describing locations
// at which the given identifier is referenced.
func (r *Finder) FindOccurrences(ident *ast.Ident) (map[string][]text.Extent, error) {

	var pkgs map[*loader.PackageInfo]bool
	var result map[string][]text.Extent

	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)

	if obj == nil && !r.IsPackageName(ident) && !r.IsSwitchVar(ident) {

		return nil, fmt.Errorf("Unable to find declaration of %s", ident.Name)
	}

	if r.IsSwitchVar(ident) {

		//fmt.Println("selected switch var inside the names")
		return r.SwitchRename(ident), nil
	}

	if r.IsPackageName(ident) {

		return r.PackageRename(ident.Name), nil
	} else {

		var decls map[types.Object]bool
		if IsMethod(obj) {
			var err error
			decls, err = r.FindDeclarationsAcrossInterfaces(ident)
			if err != nil {
				return nil, err
			}
		} else {
			decls = map[types.Object]bool{obj: true}
		}

		result = r.occurrences(decls)
		pkgs = r.packages(decls)
	}

	return r.FindInComments(ident.Name, pkgs, result), nil
}

func (r *Finder) SwitchRename(ident *ast.Ident) map[string][]text.Extent {
	//TODO change to perform switch and case variable rename
	result := r.occurrencesofCaseVar(ident.Name)
	pkgs := allPackages(r.program)
	return r.FindInComments(ident.Name, pkgs, result)

}

func (r *Finder) PackageRename(identName string) map[string][]text.Extent {

	result := r.occurrencesofpkg(identName)
	pkgs := allPackages(r.program)
	return r.FindInComments(identName, pkgs, result)

}

// occurrences returns the source locations of all identifiers that resolve
// to one of the given objects.
func (r *Finder) occurrences(decls map[types.Object]bool) map[string][]text.Extent {
	result := make(map[string][]text.Extent)
	for pkgInfo := range r.packages(decls) {
		for id, obj := range pkgInfo.Defs {
			if decls[obj] {
				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}
		for id, obj := range pkgInfo.Uses {
			if decls[obj] {
				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}
	}

	return result
}

//TODO : Make the search robust for packagenames in importspec
func (r *Finder) occurrencesofpkg(identName string) map[string][]text.Extent {

	result := make(map[string][]text.Extent)
	for pkgInfo := range allPackages(r.program) {
		for id, obj := range pkgInfo.Defs {
			if obj == nil && id.Name == identName {

				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}
		for id, obj := range pkgInfo.Uses {
			if (obj == nil || obj.Name() == identName) && id.Name == identName {

				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}

		for node, pkgObject := range pkgInfo.Implicits {
			if pkgObject.Name() == identName {

				filename := r.positionofObject(pkgObject).Filename

				result[filename] = append(result[filename],
					r.offsetLengthofObject(node, pkgObject))
			}

			for _, file := range pkgInfo.Files {

				ast.Inspect(file, func(node ast.Node) bool {
					switch n := node.(type) {
					case *ast.ImportSpec:
						if n.Name != nil && strings.Replace(n.Path.Value, "\"", "", 2) == identName {
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

//TODO : Make the search robust
func (r *Finder) occurrencesofCaseVar(identName string) map[string][]text.Extent {

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

func (r *Finder) position(id *ast.Ident) token.Position {
	return r.program.Fset.Position(id.NamePos)
}
func (r *Finder) positionofObject(pkgObject types.Object) token.Position {
	return r.program.Fset.Position(pkgObject.Pos())
}
func (r *Finder) positionofPkg(id *ast.BasicLit) token.Position {
	return r.program.Fset.Position(id.ValuePos)
}

func (r *Finder) offsetLength(id *ast.Ident) text.Extent {
	position := r.position(id)
	offset := position.Offset
	length := len(id.Name)
	return text.Extent{offset, length}
}

func (r *Finder) offsetLengthofObject(node ast.Node, obj types.Object) text.Extent {

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

func (r *Finder) offsetLengthofPkg(id *ast.BasicLit) text.Extent {

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

// packages returns a set of PackageInfos that may reference the given
// Objects.  If at least one of the given declarations is exported, the method
// returns all the packages of this program; otherwise, it returns the
// package(s) containing the given declarations.
// TODO(review D7); If performance is a concern, you could return only the
// packages in the reverse transitive closure of the package import graph,
// rather than all the packages.
func (r *Finder) packages(decls map[types.Object]bool) map[*loader.PackageInfo]bool {
	pkgs := make(map[*loader.PackageInfo]bool)
	for decl := range decls {
		if decl.Exported() {
			return allPackages(r.program)
		}
		pkgInfo := r.program.AllPackages[decl.Pkg()]
		pkgs[pkgInfo] = true
	}
	return pkgs
}

func (r *Finder) pkgInfoForPkg(pkg *types.Package) *loader.PackageInfo {
	return r.program.AllPackages[pkg]
}

func allPackages(prog *loader.Program) map[*loader.PackageInfo]bool {
	pkgs := map[*loader.PackageInfo]bool{}
	for _, pkgInfo := range prog.AllPackages {
		pkgs[pkgInfo] = true
	}
	return pkgs
}
