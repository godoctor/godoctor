// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dataflow

import (
	"go/ast"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
	"github.com/willf/bitset"
	"golang-refactoring.org/go-doctor/analysis/cfg"
)

// File defines live variables analysis for a statement
// level control flow graph. Defer has quirks, see LiveVars func.
//
// based on algo from ch 9.2, p.610 Dragonbook, v2.2,
// "Iterative algorithm to compute live variables":
//
// IN[EXIT] = use[each D in Defers];
// for(each basic block B other than EXIT) IN[B} = {};
// for(changes to any IN occur)
//    for(each basic block B other than EXIT) {
//      OUT[B] = Union(S a successor of B) IN[S];
//      IN[B] = use[b] Union (OUT[B] - def[b]);
//    }

// NOTE: for extract function: defers in the block to extract can
// (probably?) be extracted if all variables used in the defer statement are
// not live at the beginning and the end of the block to extract

// LiveAt returns the in and out set of live variables for each block in
// a given control flow graph (cfg) in the context of a loader.Program,
// including the cfg.Entry and cfg.Exit nodes.
//
// The traditional approach of holding the live variables at the exit node
// to the empty set has been deviated from in order to handle defers.
// The live variables in set of the cfg.Exit node will be set to the variables used
// in all cfg.Defers. No liveness is analyzed for the cfg.Defers themselves.
//
// More formally:
//  IN[EXIT] = USE(each d in cfg.Defers)
//  OUT[EXIT] = {}
func LiveVars(cfg *cfg.CFG, info *loader.PackageInfo) (in, out map[ast.Stmt]map[*types.Var]struct{}) {
	lvBuilder := &liveVarBuilder{
		cfg:    cfg,
		blocks: cfg.Blocks(),
		info:   info,
		ins:    make(map[ast.Stmt]*bitset.BitSet),
		outs:   make(map[ast.Stmt]*bitset.BitSet),
		def:    make(map[ast.Stmt]*bitset.BitSet),
		use:    make(map[ast.Stmt]*bitset.BitSet),
	}

	lvBuilder.buildDefUse()
	lvBuilder.build()
	return lvBuilder.results()
}

type liveVarBuilder struct {
	cfg       *cfg.CFG
	info      *loader.PackageInfo
	blocks    []ast.Stmt
	vars      []*types.Var // list of vars whose indices appear in bitsets
	ins, outs map[ast.Stmt]*bitset.BitSet
	def, use  map[ast.Stmt]*bitset.BitSet
}

// buildDefUse builds def and use bitsets
func (lv *liveVarBuilder) buildDefUse() {
	varIndices := make(map[*types.Var]uint) // map var to its index in lv.vars

	for _, block := range lv.blocks {
		// prime the def-uses sets
		lv.def[block] = new(bitset.BitSet)
		lv.use[block] = new(bitset.BitSet)

		def := defs(block, lv.info)
		use := uses(block, lv.info)

		// use[Exit] = use(each d in cfg.Defers)
		if block == lv.cfg.Exit {
			for _, d := range lv.cfg.Defers {
				use = append(use, uses(d, lv.info)...)
			}
		}

		for _, d := range def {
			// if we have it already, uses that index
			// if we don't, add it to our slice and save its index
			k, ok := varIndices[d]
			if !ok {
				k = uint(len(lv.vars))
				varIndices[d] = k
				lv.vars = append(lv.vars, d)
			}

			lv.def[block].Set(k)
		}

		for _, u := range use {
			k, ok := varIndices[u]
			if !ok {
				k = uint(len(lv.vars))
				varIndices[u] = k
				lv.vars = append(lv.vars, u)
			}

			lv.use[block].Set(k)
		}
	}
}

// build will compute the live variables for each block in the builder's cfg.
// Precondition: buildDefUse() must have been called previously.
func (lv *liveVarBuilder) build() {
	// for(each basic block B) IN[B} = {};
	for _, block := range lv.blocks {
		lv.ins[block] = new(bitset.BitSet)
		lv.outs[block] = new(bitset.BitSet)
	}

	// for(changes to any IN occur)
	for {
		var change bool

		// for(each basic block B) {
		for _, block := range lv.blocks {

			// OUT[B] = Union(S a succ of B) IN[S]
			for _, s := range lv.cfg.Succs(block) {
				lv.outs[block].InPlaceUnion(lv.ins[s])
			}

			old := lv.ins[block].Clone()

			// IN[B] = uses[B] U (OUT[B] - def[B])
			lv.ins[block] = lv.use[block].Union(lv.outs[block].Difference(lv.def[block]))

			change = change || !old.Equal(lv.ins[block])
		}

		if !change {
			break
		}
	}
}

// results maps bits from the in and out sets back to the corresponding vars.
// Precondition: build() must have been called previously.
func (lv *liveVarBuilder) results() (in, out map[ast.Stmt]map[*types.Var]struct{}) {
	in = make(map[ast.Stmt]map[*types.Var]struct{})
	out = make(map[ast.Stmt]map[*types.Var]struct{})

	for _, block := range lv.blocks {
		in[block] = make(map[*types.Var]struct{})
		out[block] = make(map[*types.Var]struct{})

		for i := uint(0); i < lv.ins[block].Len(); i++ {
			if lv.ins[block].Test(i) {
				in[block][lv.vars[i]] = struct{}{}
			}
		}

		for i := uint(0); i < lv.outs[block].Len(); i++ {
			if lv.outs[block].Test(i) {
				out[block][lv.vars[i]] = struct{}{}
			}
		}
	}
	return in, out
}
