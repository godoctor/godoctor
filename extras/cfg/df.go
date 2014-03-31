// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains data flow analyses that can be performed on a
// previously constructed control flow graph. For details, dig in.

package cfg

import (
	"go/ast"

	"github.com/willf/bitset"
)

// TODO(reed): think this is a nice thought and all but when will we ever
// not directly need the underlying type? Maybe readability? idk... don't be a java programmer
type DFbuilder interface {
	buildGenKill()
	build()
}

// cfg is a constructed CFG to analyze.
//
// blocks is a slice of the blocks in cfg.bMap, except the START,
// for enumerating (i.e. indexing)
//
// gen, kill are each blocks' GEN and KILL bitsets.
type reachingBuilder struct {
	cfg       *CFG
	blocks    []*block
	gen, kill map[*block]*bitset.BitSet
}

// TODO(reed): DRY...
// cfg is a constructed CFG to analyze.
//
// blocks is a slice of the blocks in cfg.bMap, except the END,
// for enumerating (i.e. indexing).
//
// objs is a list of all unique ast.Objects found in a CFG, for indexing in bitset.
//
// def, use are each blocks' DEF and USE bitsets.
type liveVarBuilder struct {
	cfg      *CFG
	blocks   []*block
	objs     []*ast.Object
	def, use map[*block]*bitset.BitSet
}

// reaching is the result of building reaching definitions.
type reaching struct {
	in  map[*block][]ast.Stmt
	out map[*block][]ast.Stmt
}

// liveVars is the result of building live variable analysis.
type liveVars struct {
	in  map[*block][]*ast.Object
	out map[*block][]*ast.Object
}

// buildReaching builds reaching definitions for a given control flow graph,
// returning a pointer to a reaching object which contains its
// IN and OUT as a slice of stmts for each block.
func buildReaching(c *CFG) *reaching {
	var blocks []*block
	for _, block := range c.bMap {
		blocks = append(blocks, block)
	}
	reach := &reachingBuilder{
		c,
		blocks,
		make(map[*block]*bitset.BitSet),
		make(map[*block]*bitset.BitSet),
	}

	reach.buildGenKill()
	return reach.build()
}

// buildLiveVars builds live variables for a given control flow graph,
// returning a pointer to a livesVars object which contains its
// IN and OUT as a slice of objects for each block, where each
// object has information about its name and declaration.
func buildLiveVars(c *CFG) *liveVars {
	var blocks []*block
	for _, block := range c.bMap {
		blocks = append(blocks, block)
	}
	lvBuilder := &liveVarBuilder{
		c,
		blocks,
		nil,
		make(map[*block]*bitset.BitSet),
		make(map[*block]*bitset.BitSet),
	}

	lvBuilder.buildDefUse()
	return lvBuilder.build()
}

// buildGenKill builds the GEN and KILL bitsets for each block in a builder's cfg.
// Used to compute reaching definitions
//
// The GEN set contains all the definitions inside the block that are
// "visible" immediately after the block -- we refer to them as downwards exposed.
//
// The KILL set is simply the union of all the definitions killed by the
// individual statements.
func (r *reachingBuilder) buildGenKill() {
	okills := make(map[*ast.Object]*bitset.BitSet)

	// prime the gen-kill sets
	for _, b := range r.blocks {
		r.gen[b] = bitset.New(0)
		r.kill[b] = bitset.New(0)
	}

	oind := 0

	// Iterate over all blocks twice, because a block may not know the entirety of what
	// it kills until all blocks have been iterated over.
	for i := 0; i < 2; i++ {
		for j, block := range r.blocks {
			j := uint(j)

			// extracts left hand side idents for Assign and Range only
			defs := extractDefStmtIdents(block.stmt)

			for _, d := range defs {
				if _, ok := okills[d.Obj]; !ok {
					okills[d.Obj] = bitset.New(0)
				}
				r.gen[block].Set(j)  // GEN this obj
				okills[d.Obj].Set(j) // KILL this obj for everyone else
				// our kills are KILL[obj] - GEN[B]
				r.kill[block] = r.kill[block].Union(okills[d.Obj]).Difference(r.gen[block])
				oind++
			}
		}
	}
}

// buildReaching will compute the reaching definitions for each block
// in the builder's cfg. precondition: buildGenKill() must have been called
// previously.
//
// algo from ch 9.2, p.607 Dragonbook, v2.2,
// "Iterative algorithm to compute reaching definitions":
//
// OUT[ENTRY] = {};
// for(each basic block B other than ENTRY) OUT[B} = {};
// for(changes to any OUT occur)
//    for(each basic block B other than ENTRY) {
//      IN[B] = Union(P a pred of B) OUT[P];
//      OUT[B] = gen[b] Union (IN[B] - kill[b]);
//    }
//
// TODO(reed): refactor refactor refactor
func (r *reachingBuilder) build() *reaching {

	ins := make(map[*block]*bitset.BitSet)
	outs := make(map[*block]*bitset.BitSet)

	// all blocks except START
	blocks := make([]*block, 0, len(r.blocks)-1)

	// OUT[ENTRY] = {};
	// for(each basic block B other than ENTRY) OUT[B} = {};
	for _, block := range r.blocks {
		ins[block] = bitset.New(0)
		outs[block] = bitset.New(0)
		if block != r.cfg.bMap[r.cfg.start] {
			blocks = append(blocks, block)
		}
	}

	// for(changes to any OUT occur)
	for change := true; change; { // best do-while impersonation I got
		change = false

		// for(each basic block B other than ENTRY) {
		for _, block := range blocks {

			// IN[B] = Union(P a pred of B) OUT[P];
			for _, p := range block.preds {
				p := r.cfg.bMap[p] // *block
				ins[block].InPlaceUnion(outs[p])
			}

			old := outs[block].Clone()

			// OUT[B] = gen[b] Union (IN[B] - kill[b]);
			outs[block] = r.gen[block].Union(ins[block].Difference(r.kill[block]))

			change = change || !old.Equal(outs[block])
		}
	}

	in := make(map[*block][]ast.Stmt)
	out := make(map[*block][]ast.Stmt)

	// map bits in IN and OUT back to corresponding blocks (with START)
	for _, block := range r.blocks {
		for i, ok := uint(0), true; ok; i++ {
			if i, ok = ins[block].NextSet(i); ok {
				in[block] = append(in[block], r.blocks[i].stmt)
			}
		}

		for i, ok := uint(0), true; ok; i++ {
			if i, ok = outs[block].NextSet(i); ok {
				out[block] = append(out[block], r.blocks[i].stmt)
			}
		}
	}
	return &reaching{in, out}
}

// goal: map bits back to objects
// builds DEF and USE bitsets for use in liveVarBuilder.build()
//
// def[B] as the set of variables _defined_ (i.e., definitely
// assigned values in B prior to any use of that variable in B, and
//
// use[B] as the set of variables whose values may be used in B prior
// to any defintion of the variable
func (lv *liveVarBuilder) buildDefUse() {
	// prime the gen-kill sets
	for _, b := range lv.blocks {
		lv.def[b] = bitset.New(0)
		lv.use[b] = bitset.New(0)
	}

	objIndices := make(map[*ast.Object]uint)

	// Iterate over all blocks twice, because a block may not know the entirety of what
	// it kills until all blocks have been iterated over.
	for i := 0; i < 2; i++ {
		for _, block := range lv.blocks {
			// DEF, USE idents... we need objects
			defs, uses := extractDefUse(block.stmt)

			for _, d := range defs {
				// if we have it already, use that index
				// if we don't, add it to our slice and save its index
				k, ok := objIndices[d.Obj]
				if !ok {
					k = uint(len(lv.objs))
					objIndices[d.Obj] = k
					lv.objs = append(lv.objs, d.Obj)
				}

				lv.def[block].Set(k)
			}

			for _, u := range uses {
				// if we have it already, use that index
				// if we don't, add it to our slice and save its index (e.g. func args)
				k, ok := objIndices[u.Obj]
				if !ok {
					k = uint(len(lv.objs))
					objIndices[u.Obj] = k
					lv.objs = append(lv.objs, u.Obj)
				}

				lv.use[block].Set(k)
			}
		}
	}
}

// build will compute the live variables for each block
// in the builder's cfg.
// precondition: buildDefUse() must have been called previously.
//
// algo from ch 9.2, p.610 Dragonbook, v2.2,
// "Iterative algorithm to compute live variables":
//
// IN[EXIT] = {};
// for(each basic block B other than EXIT) IN[B} = {};
// for(changes to any IN occur)
//    for(each basic block B other than EXIT) {
//      OUT[B] = Union(S a successor of B) IN[S];
//      IN[B] = use[b] Union (OUT[B] - def[b]);
//    }
//
// TODO(reed): refactor refactor refactor
func (lv *liveVarBuilder) build() *liveVars {

	// each bitset will map to a bit that represents an *ast.Object
	ins := make(map[*block]*bitset.BitSet)
	outs := make(map[*block]*bitset.BitSet)

	// mblocks will be all of the cfg blocks except EXIT
	blocks := make([]*block, 0, len(lv.blocks)-1)

	// IN[EXIT] = {};
	// for(each basic block B other than EXIT) IN[B} = {};
	for _, block := range lv.blocks {
		ins[block] = bitset.New(0)
		outs[block] = bitset.New(0)
		if block != lv.cfg.bMap[lv.cfg.end] {
			blocks = append(blocks, block)
		}
	}

	// for(changes to any IN occur)
	for change := true; change; { // best do-while impersonation I got
		change = false

		// for(each basic block B other than EXIT) {
		for _, block := range blocks {

			// OUT[B] = Union(S a succ of B) IN[S]
			for _, s := range block.succs {
				s := lv.cfg.bMap[s]
				outs[block].InPlaceUnion(ins[s])
			}

			old := ins[block].Clone()

			// IN[B] = use[B] U (OUT[B] - def[B])
			ins[block] = lv.use[block].Union(outs[block].Difference(lv.def[block]))

			change = change || !old.Equal(ins[block])
		}
	}

	in := make(map[*block][]*ast.Object)
	out := make(map[*block][]*ast.Object)

	// map bits in IN and OUT back to corresponding objects
	for _, block := range lv.blocks {
		for i := uint(0); i < ins[block].Len(); i++ {
			if ins[block].Test(i) {
				in[block] = append(in[block], lv.objs[i])
			}
		}

		for i := uint(0); i < outs[block].Len(); i++ {
			if outs[block].Test(i) {
				out[block] = append(out[block], lv.objs[i])
			}
		}
	}
	return &liveVars{in, out}
}

// Extracts the DEF and USE sets of variables for a given list of statements.
//
// def as the set of variables _defined_ (i.e., definitely
// assigned values in B prior to any use of that variable in B, and
//
// use as the set of variables whose values may be used in B prior
// to any defintion of the variable
//
// i.e. DEF is LHS identifiers only, USE is all identifiers NOT on the LHS
func ExtractDefUse(stmts []ast.Stmt) (def []*ast.Object, use []*ast.Object) {
	defmap := make(map[*ast.Object]bool)
	usemap := make(map[*ast.Object]bool)

	for _, stmt := range stmts {
		// DEF and USE identifiers, we need objects
		defs, uses := extractDefUse(stmt)

		// find idents in LHS expressions, add to DEF
		for _, d := range defs {
			if _, ok := defmap[d.Obj]; !ok {
				defmap[d.Obj] = true
				def = append(def, d.Obj)
			}
		}

		for _, u := range uses {
			if _, ok := usemap[u.Obj]; !ok {
				usemap[u.Obj] = true
				use = append(use, u.Obj)
			}
		}
	}
	return use, def
}

// for brevity's sake, return both
func extractDefUse(stmt ast.Stmt) (def []*ast.Ident, use []*ast.Ident) {
	return extractDefStmtIdents(stmt), extractUseStmtIdents(stmt)
}

func extractDefStmtIdents(stmt ast.Stmt) []*ast.Ident {
	var idents []*ast.Ident
	var defs []ast.Expr
	// extract all LHS expressions to look for idents
	switch stmt := stmt.(type) {
	// TODO other stmts we need? Decl?
	case *ast.AssignStmt:
		defs = stmt.Lhs
	case *ast.RangeStmt:
		if stmt.Key != nil {
			defs = append(defs, stmt.Key)
		}
		if stmt.Value != nil {
			defs = append(defs, stmt.Value)
		}
	}

	// find idents in LHS expressions, add to DEF
	for _, d := range defs {
		idents = append(idents, extractExprIdents(d)...)
	}
	return idents
}

// Extracts USE idents from a given statement.
// i.e. Assignment and Range only produce their RHS idents.
func extractUseStmtIdents(stmt ast.Stmt) []*ast.Ident {
	var idents []*ast.Ident
	var exprs []ast.Expr

	switch stmt := stmt.(type) {
	//case *ast.AssignStmt: // THESE ARE USEFUL FOR LHS U RHS IDENTS
	//exprs = append(stmt.Lhs, stmt.Rhs...)
	//case *ast.RangeStmt: // Body sent separate
	//exprs = append(exprs, stmt.Key, stmt.Value, stmt.X)
	case *ast.AssignStmt:
		exprs = stmt.Rhs
	case *ast.RangeStmt: // Body sent separate
		exprs = append(exprs, stmt.X)
	case *ast.BranchStmt:
		idents = append(idents, stmt.Label)
	case *ast.CaseClause: // Body sent separate
		exprs = stmt.List
	case *ast.CommClause: // Body & Comm sent separate
	case *ast.DeferStmt:
		exprs = append(exprs, stmt.Call)
	case *ast.ExprStmt:
		exprs = append(exprs, stmt.X)
	case *ast.ForStmt: // Init, Body & Post sent separate
		exprs = append(exprs, stmt.Cond)
	case *ast.GoStmt:
		exprs = append(exprs, stmt.Call)
	case *ast.IfStmt: // Init, Body, & Else sent separate
		exprs = append(exprs, stmt.Cond)
	case *ast.IncDecStmt:
		exprs = append(exprs, stmt.X)
	case *ast.LabeledStmt:
		idents = append(idents, stmt.Label)
	case *ast.ReturnStmt:
		exprs = stmt.Results
	case *ast.SendStmt:
		exprs = append(exprs, stmt.Chan, stmt.Value)
	case *ast.SwitchStmt: // Init & Body sent separate
		exprs = append(exprs, stmt.Tag)
	}
	for _, e := range exprs {
		idents = append(idents, extractExprIdents(e)...)
	}
	return idents
}

// An expression is represented by a tree consisting of one
// or more of the following concrete expression nodes.
// This extracts the variable idents from the expression.
func extractExprIdents(expr ast.Expr) []*ast.Ident {
	var idents []*ast.Ident
	var exprs []ast.Expr

	switch expr := expr.(type) {
	case *ast.Ident:
		idents = append(idents, expr)
	case *ast.BinaryExpr:
		exprs = append(exprs, expr.X, expr.Y)
	case *ast.CallExpr:
		//exprs = append(expr.Args, expr.Fun) // TODO only need Fun w/ decl in scope...
		exprs = expr.Args
	case *ast.IndexExpr:
		exprs = append(exprs, expr.X, expr.Index)
	case *ast.ParenExpr:
		exprs = append(exprs, expr.X)
	case *ast.SelectorExpr:
		exprs = append(exprs, expr.X)
		idents = append(idents, expr.Sel)
	case *ast.SliceExpr:
		exprs = append(exprs, expr.X, expr.Low, expr.High, expr.Max)
	case *ast.StarExpr:
		exprs = append(exprs, expr.X)
	case *ast.TypeAssertExpr:
		exprs = append(exprs, expr.X)
	case *ast.UnaryExpr:
		exprs = append(exprs, expr.X)
	case *ast.CompositeLit:
		exprs = expr.Elts
	case *ast.Ellipsis: // TODO what is Elt and don't we have it already?
		exprs = append(exprs, expr.Elt)
	case *ast.KeyValueExpr: // TODO isn't Key a Field?
		exprs = append(exprs, expr.Key, expr.Value)
	default:
		return nil // TODO ?
		// case *ast.FuncLit: // only function names
	}
	for _, e := range exprs {
		idents = append(idents, extractExprIdents(e)...)
	}

	for i := 0; i < len(idents); i++ {
		ident := idents[i]
		if ident.Name == "_" { // remove these
			idents = append(idents[:i], idents[i+1:]...)
			i--
		}
	}

	return idents
}
