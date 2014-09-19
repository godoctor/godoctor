// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
)

/* -=-=- Search for Conflicting Declarations -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// FindConflict determines if there already exists an identifier with the given
// newName such that the given ident cannot be renamed to newName.  It returns
// the first such conflicting declaration, if one exists, and nil otherwise.
func FindConflict(ident *ast.Ident, pkgInfo *loader.PackageInfo, newName string) *ast.Ident {
	obj := pkgInfo.ObjectOf(ident)

	if obj == nil && !IsPackageName(ident, pkgInfo) && !isSwitchVar(ident, pkgInfo) {
		return ident
	}

	if IsPackageName(ident, pkgInfo) || isSwitchVar(ident, pkgInfo) {
		return nil
	}

	if obj.Parent() != nil {
		if result := findConflictInChildScope(ident, obj.Parent(), newName); result != nil {
			return result
		}
	}

	if IsMethod(obj) {
		objfound, _, pointerindirections := types.LookupFieldOrMethod(MethodReceiver(obj).Type(), true, obj.Pkg(), newName)
		if IsMethod(objfound) && pointerindirections {
			return ident
		} else {
			return nil
		}
	}

	if obj.Parent().LookupParent(newName) != nil {
		return ident
	}

	return nil
}

func findConflictInChildScope(ident *ast.Ident, identScope *types.Scope, newName string) *ast.Ident {
	if identScope.Lookup(newName) != nil {
		return ident
	}

	for i := 0; i < identScope.NumChildren(); i++ {
		if result := findConflictInChildScope(ident, identScope.Child(i), newName); result != nil {
			return result
		}
	}
	return nil
}
