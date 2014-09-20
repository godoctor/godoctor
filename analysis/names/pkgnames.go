// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"
	"strings"

	"code.google.com/p/go.tools/go/loader"
)

//TODO : Make the search robust for packagenames in importspec
func FindReferencesToPackage(pkgName string, program *loader.Program) (idents map[*ast.Ident]bool, imports map[*ast.ImportSpec]bool) {
	idents = map[*ast.Ident]bool{}
	imports = map[*ast.ImportSpec]bool{}
	for pkgInfo := range allPackages(program) {
		for id, obj := range pkgInfo.Defs {
			if obj == nil && id.Name == pkgName {
				idents[id] = true
			}
		}

		for id, obj := range pkgInfo.Uses {
			if (obj == nil || obj.Name() == pkgName) && id.Name == pkgName {
				idents[id] = true
			}
		}

		for _, file := range pkgInfo.Files {
			for _, importSpec := range file.Imports {
				pkgObj := pkgInfo.Implicits[importSpec]
				if pkgObj == nil && strings.Contains(importSpec.Path.Value, pkgName) ||
					pkgObj != nil && pkgObj.Name() == pkgName {
					imports[importSpec] = true
				}
			}
		}
	}
	return
}
