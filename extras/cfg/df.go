// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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

type reachingBuilder struct {
	cfg       *CFG
	blocks    []*block
	gen, kill map[*block]*bitset.BitSet
}

// TODO(reed): DRY...
type liveVarBuilder struct {
	cfg      *CFG
	blocks   []*block
	objs     []*ast.Object
	def, use map[*block]*bitset.BitSet
}

type reaching struct {
	in  map[*block][]ast.Stmt
	out map[*block][]ast.Stmt
}

type liveVars struct {
	in  map[*block][]*ast.Object
	out map[*block][]*ast.Object
}

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
func (r *reachingBuilder) buildGenKill() {
	okills := make(map[*ast.Object]*bitset.BitSet)

	// prime the gen-kill sets
	for _, b := range r.blocks {
		r.gen[b] = bitset.New(0)
		r.kill[b] = bitset.New(0)
	}

	// Iterate over all blocks twice, because a block may not know the entirety of what
	// it kills until all blocks have been iterated over.
	oind := 0

	for i := 0; i < 2; i++ {
		for j, block := range r.blocks {
			j := uint(j)
			var exprs []ast.Expr
			// extract all "producing" statements; assignments
			switch stmt := block.stmt.(type) {
			// TODO other stmts we need? Decl?
			case *ast.AssignStmt:
				exprs = stmt.Lhs
			case *ast.RangeStmt:
				if stmt.Key != nil {
					exprs = append(exprs, stmt.Key)
				}
				if stmt.Value != nil {
					exprs = append(exprs, stmt.Value)
				}
			}

			for _, e := range exprs {
				switch e := e.(type) {
				// TODO other types of exprs?
				case *ast.Ident:
					if e.Name == "_" {
						continue
					}
					if _, ok := okills[e.Obj]; !ok {
						okills[e.Obj] = bitset.New(0)
					}
					r.gen[block].Set(j)
					okills[e.Obj].Set(j)
					r.kill[block] = r.kill[block].Union(okills[e.Obj]).Difference(r.gen[block])
					oind++
				}
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

// TODO goal: map bits back to objects, not statements
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
			var defs []ast.Expr
			switch stmt := block.stmt.(type) {
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

			for _, e := range defs {
				switch e := e.(type) {
				// TODO other types of exprs?
				case *ast.Ident:
					if e.Name == "_" {
						continue
					}
					k, ok := objIndices[e.Obj]
					if !ok {
						k = uint(len(lv.objs))
						objIndices[e.Obj] = k
						lv.objs = append(lv.objs, e.Obj)
					}
					lv.def[block].Set(k)
				}
			}

			uses := ExtractUseStmtIdents(block.stmt)

			for _, u := range uses {
				if u.Name == "_" { // TODO is this possible?
					continue
				}
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

func ExtractUseStmtIdents(stmt ast.Stmt) []*ast.Ident {
	var idents []*ast.Ident
	var exprs []ast.Expr

	switch stmt := stmt.(type) {
	//case *ast.AssignStmt:
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
		idents = append(idents, ExtractExprIdents(e)...)
	}
	return idents
}

// An expression is represented by a tree consisting of one
// or more of the following concrete expression nodes.
//
// TODO(reed): find out if this runs 4 eva
func ExtractExprIdents(expr ast.Expr) []*ast.Ident {
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
		idents = append(idents, ExtractExprIdents(e)...)
	}

	return idents
}

// buildReaching will compute the reaching definitions for each block
// in the builder's cfg. precondition: buildGenKill() must have been called
// previously.
//
// algo from ch 9.2, p.610 Dragonbook, v2.2,
// "Iterative algorithm to compute reaching definitions":
//
// IN[EXIT] = {};
// for(each basic block B other than EXIT) IN[B} = {};
// for(changes to any IN occur)
//    for(each basic block B other than EXIT) {
//      OUT[B] = Union(S a successor of B) IN[S];
//      IN[B] = gen[b] Union (OUT[B] - kill[b]);
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
	// for(each basic block B other than ENTRY) OUT[B} = {};
	for _, block := range lv.blocks {
		ins[block] = bitset.New(0)
		outs[block] = bitset.New(0)
		if block != lv.cfg.bMap[lv.cfg.end] {
			blocks = append(blocks, block)
		}
	}

	// for(changes to any OUT occur)
	for change := true; change; { // best do-while impersonation I got
		change = false

		// for(each basic block B other than EXIT) {
		for _, block := range blocks {

			// IN[B] = Union(P a pred of B) OUT[P];
			for _, s := range block.succs {
				s := lv.cfg.bMap[s]
				outs[block].InPlaceUnion(ins[s])
			}

			old := ins[block].Clone()

			// OUT[B] = gen[b] Union (IN[B] - kill[b]);
			ins[block] = lv.def[block].Union(outs[block].Difference(lv.use[block]))

			change = change || !old.Equal(ins[block])
		}
	}

	in := make(map[*block][]*ast.Object)
	out := make(map[*block][]*ast.Object)

	// map bits in IN and OUT back to corresponding statements
	for _, block := range lv.blocks {
		for i, ok := uint(0), true; ok; i++ {
			if i, ok = ins[block].NextSet(i); ok {
				in[block] = append(in[block], lv.objs[i])
			}
		}

		for i, ok := uint(0), true; ok; i++ {
			if i, ok = outs[block].NextSet(i); ok {
				out[block] = append(out[block], lv.objs[i])
			}
		}
	}
	return &liveVars{in, out}
}
