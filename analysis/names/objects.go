// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
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

func allPackages(prog *loader.Program) map[*loader.PackageInfo]bool {
	result := map[*loader.PackageInfo]bool{}
	for _, pkgInfo := range prog.AllPackages {
		result[pkgInfo] = true
	}
	return result
}
