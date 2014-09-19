// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"

	"code.google.com/p/go.tools/go/types"
)

/* -=-=- Search for Conflicting Declarations -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// FindConflict determines if there already exists an identifier with the given
// newName such that the given ident cannot be renamed to newName.  It returns
// the first such conflicting declaration, if one exists, and nil otherwise.
func (r *Finder) FindConflict(ident *ast.Ident, newName string) *ast.Ident {
	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)

	if obj == nil && !r.IsPackageName(ident) && !r.IsSwitchVar(ident) {
		return ident
	}

	if r.IsPackageName(ident) || r.IsSwitchVar(ident) {
		return nil
	}

	if obj.Parent() != nil {
		if result := r.findConflictInChildScope(ident, obj.Parent(), newName); result != nil {
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

func (r *Finder) findConflictInChildScope(ident *ast.Ident, identScope *types.Scope, newName string) *ast.Ident {
	//fmt.Println("child scope",  identScope.String(), identScope.Names(), identScope.NumChildren())
	if identScope.Lookup(newName) != nil {
		return ident
	}

	for i := 0; i < identScope.NumChildren(); i++ {
		if result := r.findConflictInChildScope(ident, identScope.Child(i), newName); result != nil {
			return result
		}
	}
	return nil
}
