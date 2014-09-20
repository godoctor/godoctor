// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"
	"go/token"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
)

func FindTypeSwitchOccurrences(typeSwitch *ast.TypeSwitchStmt, pkgInfo *loader.PackageInfo) map[*ast.Ident]bool {
	result := make(map[*ast.Ident]bool)

	// Add v from "switch v := e.(type)"
	if asgt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
		if len(asgt.Lhs) == 1 && asgt.Tok == token.DEFINE {
			if id, ok := asgt.Lhs[0].(*ast.Ident); ok {
				result[id] = true
			}
		}
	}

	// Collect the implicit *types.Var defined by each case clause
	caseClauseVars := map[*types.Var]bool{}
	for _, stmt := range typeSwitch.Body.List {
		obj := pkgInfo.Implicits[stmt.(*ast.CaseClause)].(*types.Var)
		caseClauseVars[obj] = true
	}

	// Find references to the implicit *types.Var for each case clause
	ast.Inspect(typeSwitch.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := pkgInfo.ObjectOf(id).(*types.Var); ok {
				if caseClauseVars[v] {
					result[id] = true
				}
			}
		}
		return true
	})

	return result
}
