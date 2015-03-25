// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dataflow provides data flow analyses that can be performed on a
// previously constructed control flow graph, including a reaching definitions
// analysis and a live variables analysis for local variables.
package dataflow

// This file contains functions common to all data flow analyses, as well as
// one exported function.

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

// ReferencedVars returns the sets of local variables that are defined or used
// within the given list of statements (based on syntax).
func ReferencedVars(stmts []ast.Stmt, info *loader.PackageInfo) (def, use map[*types.Var]struct{}) {
	def = make(map[*types.Var]struct{})
	use = make(map[*types.Var]struct{})

	for _, stmt := range stmts {
		for _, d := range defs(stmt, info) {
			def[d] = struct{}{}
		}
		for _, u := range uses(stmt, info) {
			use[u] = struct{}{}
		}
	}
	return def, use
}

// defs extracts any local variables whose values are assigned in the given statement.
func defs(stmt ast.Stmt, info *loader.PackageInfo) []*types.Var {
	idnts := make(map[*ast.Ident]struct{})

	switch stmt := stmt.(type) {
	case *ast.DeclStmt: // vars (1+) in decl; zero values
		ast.Inspect(stmt, func(n ast.Node) bool {
			if v, ok := n.(*ast.ValueSpec); ok {
				idnts = union(idnts, idents(v))
			}
			return true
		})
	case *ast.IncDecStmt: // i++, i--
		idnts = idents(stmt.X)
	case *ast.AssignStmt: // :=, =, &=, etc. except x[i] (IndexExpr)
		for _, x := range stmt.Lhs {
			indExp := false
			ast.Inspect(x, func(n ast.Node) bool {
				if _, ok := n.(*ast.IndexExpr); ok {
					indExp = true
					return false
				}
				return true
			})
			if !indExp {
				idnts = union(idnts, idents(x))
			}
		}
	case *ast.RangeStmt: // only [ x, y ] on Lhs
		idnts = union(idents(stmt.Key), idents(stmt.Value))
	case *ast.TypeSwitchStmt:
		// The assigned variable does not have a types.Var
		// associated in this stmt; rather, the uses of that
		// variable in the case clauses have several different
		// types.Vars associated with them, according to type
		var vars []*types.Var
		ast.Inspect(stmt.Body, func(n ast.Node) bool {
			switch cc := n.(type) {
			case *ast.CaseClause:
				v := typeCaseVar(info, cc)
				if v != nil {
					vars = append(vars, v)
				}
				return false
			default:
				return true
			}
		})
		return vars
	}

	var vars []*types.Var
	// should all map to types.Var's, if not we don't want anyway
	for i, _ := range idnts {
		if v, ok := info.ObjectOf(i).(*types.Var); ok {
			vars = append(vars, v)
		}
	}
	return vars
}

// typeCaseVar returns the implicit variable associated with a case clause in a
// type switch statement.
func typeCaseVar(info *loader.PackageInfo, cc *ast.CaseClause) *types.Var {
	// Removed from go/loader
	if v := info.Implicits[cc]; v != nil {
		return v.(*types.Var)
	}
	return nil
}

// uses extracts local variables whose values are used in the given statement.
func uses(stmt ast.Stmt, info *loader.PackageInfo) []*types.Var {
	idnts := make(map[*ast.Ident]struct{})

	ast.Inspect(stmt, func(n ast.Node) bool {
		switch stmt := stmt.(type) {
		case *ast.AssignStmt: // mostly rhs of =, :=, &=, etc.
			// some LHS are uses, e.g. x[i]
			for _, x := range stmt.Lhs {
				indExp := false
				ast.Inspect(stmt, func(n ast.Node) bool {
					if _, ok := n.(*ast.IndexExpr); ok {
						indExp = true
						return false
					}
					return true
				})
				if indExp || // x[i] is a uses of x and i
					(stmt.Tok != token.ASSIGN &&
						stmt.Tok != token.DEFINE) { // e.g. +=, ^=, etc.
					idnts = union(idnts, idents(x))
				}
			}
			// all RHS are uses
			for _, s := range stmt.Rhs {
				idnts = union(idnts, idents(s))
			}
		case *ast.BlockStmt: // no uses, skip - should not appear in cfg
		case *ast.BranchStmt: // no uses, skip
		case *ast.CaseClause: // no uses, skip
		case *ast.CommClause: // no uses, skip
		case *ast.DeclStmt: // no uses, skip
		case *ast.DeferStmt:
			idnts = idents(stmt.Call)
		case *ast.ForStmt:
			idnts = idents(stmt.Cond)
		case *ast.IfStmt:
			idnts = idents(stmt.Cond)
		case *ast.LabeledStmt: // no uses, skip
		case *ast.RangeStmt: // list in _, _ = range [ list ]
			idnts = idents(stmt.X)
		case *ast.SelectStmt: // no uses, skip
		case *ast.SwitchStmt:
			idnts = idents(stmt.Tag)
		case *ast.TypeSwitchStmt: // no uses, skip
		case ast.Stmt: // everything else is all uses
			idnts = idents(stmt)
		}
		return true
	})

	var vars []*types.Var

	// should all map to types.Var's, if not we don't want anyway
	for i, _ := range idnts {
		if v, ok := info.ObjectOf(i).(*types.Var); ok {
			vars = append(vars, v)
		}
	}

	return vars
}

// idents returns the set of all identifiers in given node.
func idents(node ast.Node) map[*ast.Ident]struct{} {
	idents := make(map[*ast.Ident]struct{})
	if node == nil {
		return idents
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			idents[n] = struct{}{}
		}
		return true
	})
	return idents
}

func union(one, two map[*ast.Ident]struct{}) map[*ast.Ident]struct{} {
	for o, _ := range one {
		two[o] = struct{}{}
	}
	return two
}
