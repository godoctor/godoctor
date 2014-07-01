package dataflow

import (
	"go/ast"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
	"github.com/willf/bitset"
	"golang-refactoring.org/go-doctor/extras/cfg"
)

// File defines reaching definitions for a statement level
// control flow graph.
//
// based on algo from ch 9.2, p.607 Dragonbook, v2.2,
// "Iterative algorithm to compute reaching definitions":
//
// OUT[ENTRY] = {};
// for(each basic block B other than ENTRY) OUT[B] = {};
// for(changes to any OUT occur)
//    for(each basic block B other than ENTRY) {
//      IN[B] = Union(P a pred of B) OUT[P];
//      OUT[B] = gen[b] Union (IN[B] - kill[b]);
//    }

// ReachingDefs builds reaching definitions for a given control flow graph, returning the
// in and out sets in a map of stmts for each block (statement).
//
// No nodes from the cfg.Defers list will be returned in the output of
// this function as they are disjoint from a cfg's blocks.
// For analyzing the statements in the cfg.Defers list, each defer
// should be treated as though it has the same in and out sets as the cfg.Exit node.
func ReachingDefs(cfg *cfg.CFG, info *loader.PackageInfo) (in, out map[ast.Stmt]map[ast.Stmt]struct{}) {
	reach := &reachingBuilder{
		cfg:    cfg,
		blocks: cfg.Blocks(),
		info:   info,
		gen:    make(map[ast.Stmt]*bitset.BitSet),
		kill:   make(map[ast.Stmt]*bitset.BitSet),
		ins:    make(map[ast.Stmt]*bitset.BitSet),
		outs:   make(map[ast.Stmt]*bitset.BitSet),
	}

	reach.buildGenKill()
	reach.build()
	return reach.results()
}

type reachingBuilder struct {
	cfg       *cfg.CFG
	info      *loader.PackageInfo
	blocks    []ast.Stmt
	gen, kill map[ast.Stmt]*bitset.BitSet
	ins, outs map[ast.Stmt]*bitset.BitSet
}

// buildGenKill builds the gen and kill bitsets for each block in a builder's cfg.
// Used to compute reaching definitions.
func (r *reachingBuilder) buildGenKill() {
	okills := make(map[*types.Var]*bitset.BitSet)

	for _, b := range r.blocks { // prime
		r.gen[b] = new(bitset.BitSet)
		r.kill[b] = new(bitset.BitSet)
	}

	// Iterate over all blocks twice, because a block may not know the entirety of what
	// it kills until all blocks have been iterated over.
	for i := 0; i < 2; i++ {
		for j, block := range r.blocks {
			j := uint(j)

			def := defs(block, r.info)

			for _, d := range def {
				if _, ok := okills[d]; !ok {
					okills[d] = new(bitset.BitSet)
				}
				r.gen[block].Set(j) // GEN this obj
				okills[d].Set(j)    // KILL this obj for everyone else
				// our kills are KILL[obj] - GEN[B]
				r.kill[block] = r.kill[block].Union(okills[d]).Difference(r.gen[block])
			}
		}
	}
}

// build will compute the reaching definitions for each block in the builder's CFG.
// Precondition: buildGenKill() must have been called previously.
func (r *reachingBuilder) build() {
	// all blocks except cfg.Entry
	blocks := make([]ast.Stmt, 0, len(r.blocks)-1)

	// OUT[ENTRY] = {};
	// for(each basic block B other than ENTRY) OUT[B} = {};
	for _, block := range r.blocks {
		r.ins[block] = new(bitset.BitSet)
		r.outs[block] = new(bitset.BitSet)
		if block != r.cfg.Entry {
			blocks = append(blocks, block)
		}
	}

	// for(changes to any OUT occur)
	for {
		var changed bool

		// for(each basic block B other than ENTRY) {
		for _, block := range blocks {

			// IN[B] = Union(P a pred of B) OUT[P];
			for _, p := range r.cfg.Preds(block) {
				r.ins[block].InPlaceUnion(r.outs[p])
			}

			old := r.outs[block].Clone()

			// OUT[B] = gen[b] Union (IN[B] - kill[b]);
			r.outs[block] = r.gen[block].Union(r.ins[block].Difference(r.kill[block]))

			changed = changed || !old.Equal(r.outs[block])
		}

		if !changed {
			break
		}
	}
}

// Map bits in the in and out sets back to corresponding statements.
// Precondition: build() must have been called previously.
func (r *reachingBuilder) results() (in, out map[ast.Stmt]map[ast.Stmt]struct{}) {
	in = make(map[ast.Stmt]map[ast.Stmt]struct{})
	out = make(map[ast.Stmt]map[ast.Stmt]struct{})

	// map bits from in and out sets back to corresponding blocks (with cfg.Entry)
	for _, block := range r.blocks {
		in[block] = make(map[ast.Stmt]struct{})
		out[block] = make(map[ast.Stmt]struct{})

		for i, ok := uint(0), true; ok; i++ {
			if i, ok = r.ins[block].NextSet(i); ok {
				in[block][r.blocks[i]] = struct{}{}
			}
		}

		for i, ok := uint(0), true; ok; i++ {
			if i, ok = r.outs[block].NextSet(i); ok {
				out[block][r.blocks[i]] = struct{}{}
			}
		}
	}
	return in, out
}
