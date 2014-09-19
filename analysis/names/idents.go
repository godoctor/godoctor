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

func IsPackageName(ident *ast.Ident, pkgInfo *loader.PackageInfo) bool {
	obj := pkgInfo.ObjectOf(ident)
	if pkgInfo.Pkg.Name() == ident.Name && obj == nil {
		return true
	}

	return false
}

func isSwitchVar(ident *ast.Ident, pkgInfo *loader.PackageInfo) bool {
	obj := pkgInfo.ObjectOf(ident)
	if _, ok := obj.(*types.Var); !ok && obj == nil && !IsPackageName(ident, pkgInfo) {
		//fmt.Println("types.var of ident",v)
		//fmt.Println("selected var in switch  clasue of type switch ")
		// fmt.Println("slected  switch var and types.var is",obj.(*types.Var))
		return true
	}

	//fmt.Println(" var is not swithvar")
	return false
}

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
func FindOccurrences(ident *ast.Ident, pkgInfo *loader.PackageInfo, program *loader.Program) (map[string][]text.Extent, error) {
	f := &finder{program}
	return f.FindOccurrences(ident, pkgInfo)
}

type finder struct {
	program *loader.Program
}

func (r *finder) FindOccurrences(ident *ast.Ident, pkgInfo *loader.PackageInfo) (map[string][]text.Extent, error) {
	obj := pkgInfo.ObjectOf(ident)
	if obj == nil && !IsPackageName(ident, pkgInfo) && !isSwitchVar(ident, pkgInfo) {

		return nil, fmt.Errorf("Unable to find declaration of %s", ident.Name)
	}

	if isSwitchVar(ident, pkgInfo) {

		//fmt.Println("selected switch var inside the names")
		return r.switchRename(ident), nil
	}

	if IsPackageName(ident, pkgInfo) {

		return PackageRename(ident.Name, r.program), nil
	}

	var decls map[types.Object]bool
	var err error
	decls, err = FindDeclarationsAcrossInterfaces(ident, pkgInfo, r.program)
	if err != nil {
		return nil, err
	}

	return r.occurrences(decls), nil
}

func (r *finder) switchRename(ident *ast.Ident) map[string][]text.Extent {
	//TODO change to perform switch and case variable rename
	return r.occurrencesofCaseVar(ident.Name)
}

func PackageRename(identName string, program *loader.Program) map[string][]text.Extent {
	r := &finder{program}
	return r.occurrencesofpkg(identName)
}

// occurrences returns the source locations of all identifiers that resolve
// to one of the given objects.
func (r *finder) occurrences(decls map[types.Object]bool) map[string][]text.Extent {
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
func (r *finder) occurrencesofpkg(identName string) map[string][]text.Extent {

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

// packages returns a set of PackageInfos that may reference the given
// Objects.  If at least one of the given declarations is exported, the method
// returns all the packages of this program; otherwise, it returns the
// package(s) containing the given declarations.
// TODO(review D7); If performance is a concern, you could return only the
// packages in the reverse transitive closure of the package import graph,
// rather than all the packages.
func (r *finder) packages(decls map[types.Object]bool) map[*loader.PackageInfo]bool {
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

func (r *finder) pkgInfoForPkg(pkg *types.Package) *loader.PackageInfo {
	return r.program.AllPackages[pkg]
}

func allPackages(prog *loader.Program) map[*loader.PackageInfo]bool {
	pkgs := map[*loader.PackageInfo]bool{}
	for _, pkgInfo := range prog.AllPackages {
		pkgs[pkgInfo] = true
	}
	return pkgs
}
