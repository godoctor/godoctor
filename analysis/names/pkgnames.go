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

//TODO : Make the search robust for packagenames in importspec
func FindReferencesToPackage(pkgName string, program *loader.Program) map[string][]text.Extent {
	return (&finder{program}).findReferencesToPackage(pkgName)
}

type finder struct {
	program *loader.Program
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
