// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cfg contains an implementation for constructing a
// statement-level Control Flow Graph (CFG) for Go code.
package cfg

import (
	"go/ast"
	"go/token"

	"github.com/willf/bitset"
)

// It is intended for use to construct control flow graphs from
// an abstract syntax tree, however, any list of statements will do.
// This is done by traversing a list of statements (likely
// from a block) depth-first and creating an adjacency list,
// implemented as a hash map of blocks, with no explicit
// edge objects. Adjacent blocks are stored as predecessors
// and successors separately for control flow information.
//
// TODO(reed): I should probably provide a better description of the algorithm,
// and I will once it is stable and has been heavily refactored.

// TODO(reed): DFS Visit function?
// TODO(reed): transitive closure function?
// TODO(reed): end/begin checker funcs? end/begin exported setter funcs?
// TODO(reed): fix: defer functions are finicky, mainly due to how
// they are to be handled within conditionals/fors. The base
// case is taken care of... thought needs to be had for special cases.

// blocks are represented inside of a map of statements
// to blocks, where the block is a representation of a
// statement with knowledge of its neighbors in the graph.
// Note: typically CFG's are constructed using the idea of
// a block of statements that are flowed to in sequence, however
// in this construction each block maps to one ast.Stmt.
//
// start, end are the beginning and end nodes, and are not
// actually members of the underlying []ast.Stmt that were
// given to construct the graph. They are useful for return
// and defer statement handling as well as declaring a singular
// entry (and exit) node.
type CFG struct {
	bMap       map[ast.Stmt]*block
	start, end *ast.BadStmt
}

// MakeCFG returns the control-flow graph for the specified statements.
// Generally, these statements are assumed to have come from a *ast.BlockStmt
// so as to have some sense of "flow" but any list of statements will do.
// They will be iterated over in depth first manner.
//
// TODO(reed): don't hook up defers to end unless we have a return or end of func?
func MakeCFG(s []ast.Stmt) *CFG {
	return newCFGBuilder().build(s)
}

// FuncCFG is a convenience wrapper function for creating a CFG
// from a given function.
func FuncCFG(f *ast.FuncDecl) *CFG {
	return MakeCFG(f.Body.List)
}

// Preds returns a slice of all immediate predecessors to the given statement
// TODO if map would be more convenient, do speak
func (c *CFG) Preds(s ast.Stmt) []ast.Stmt {
	// TODO remove START/END? return err if no stmt?
	return c.bMap[s].preds
}

// Succs returns a slice of all immediate successors to the given statement
// TODO if map would be more convenient, do speak
func (c *CFG) Succs(s ast.Stmt) []ast.Stmt {
	// TODO remove START/END? return err if no stmt?
	return c.bMap[s].succs
}

func (c *CFG) Reaching(s ast.Stmt) ([]ast.Stmt, []ast.Stmt) {
	// TODO(reed): check if built, then do this if not (save this somehow)?
	// TODO(reed): fix: func params?
	return c.bMap[s].in, c.bMap[s].out
}

func (c *CFG) Live(s ast.Stmt) ([]ast.Stmt, []ast.Stmt) {
	return c.bMap[s].liveIn, c.bMap[s].liveOut
}

// Blocks are represented inside of a map of statements
// to blocks, where the block is a representation of a
// statement with knowledge of its neighbors in the graph.
//
// edges are all of the current leaf nodes that need to be
// hooked up in some manner at the next block iteration.
// e.g. if/else has 2 (typically) leaf nodes, most statements
// only produce one.
//
// branches are all of the branch statements that have been
// found while building yet are yet to be handled. In order
// to be handled properly, branches need to be passed up the tree
// until an appropriate statement has been found to handle them.
// e.g. unlabeled break inside of an if inside of a for loop
// must be handled at the block level for the for loop
//
// start, end are the beginning and end nodes, and are not
// actually members of the underlying []ast.Stmt that were
// given to construct the graph. They are useful for return
// and defer statement handling as well as declaring a singular
// entry (and exit) node.
//
// dHead, dTail are analagous to the head and tail of the defer
// stack that will be called upon return of control, directly
// after any return statements and before the actual end node.
//
// gen, kill are the GEN and KILL sets, used for reaching and
// live variable data flow analysis
type builder struct {
	bMap            map[ast.Stmt]*block
	edges, branches []ast.Stmt
	start, end      *ast.BadStmt
	dHead, dTail    *ast.DeferStmt
	blocks          []ast.Stmt
	gen, kill       map[ast.Stmt]*bitset.BitSet
}

// Create a more reasonable zero value builder; ready to go
func newCFGBuilder() *builder {
	return &builder{make(map[ast.Stmt]*block),
		nil, nil,
		&ast.BadStmt{}, &ast.BadStmt{},
		nil, nil,
		nil,
		make(map[ast.Stmt]*bitset.BitSet), make(map[ast.Stmt]*bitset.BitSet),
	}
}

// build runs buildBlock and then appropriately
// hooks up the defers (if present) and end,
// returning only the data that we need in a CFG.
func (b *builder) build(s []ast.Stmt) *CFG {
	b.buildBlock(b.start, s)
	if b.dHead != nil {
		b.flowTo(b.dTail, b.end).buildEdges(b.dHead)
	} else {
		b.buildEdges(b.end)
	}
	b.buildGenKill()
	b.buildReaching()
	b.buildLiveVariable()
	return &CFG{b.bMap, b.start, b.end}
}

// block maps directly to an ast.Stmt, with
// predecessors and successors stored separately
// for later convenience
type block struct {
	stmt    ast.Stmt
	preds   []ast.Stmt
	succs   []ast.Stmt
	in      []ast.Stmt
	out     []ast.Stmt
	liveIn  []ast.Stmt
	liveOut []ast.Stmt
}

func newBlock(s ast.Stmt) *block {
	return &block{s, nil, nil, nil, nil, nil, nil}
}

// flowTo will add an edge to the graph between the given
// src and destination statements, in the process adding
// dest as a successor to src and src a predecessor to dest.
// If dest is nil: add src as edge to handle at higher level.
func (b *builder) flowTo(src, dest ast.Stmt) *builder {
	if dest == nil {
		b.edges = append(b.edges, src)
		return b
	}
	v := b.block(src)
	w := b.block(dest)

	// allows us to keep track of end and begin's succs/preds
	// without returning them as badStmt
	// TODO(reed): gotta be a way to exlude these suckers
	//if src == b.start || src == b.end {
	//src = nil
	//}
	//if dest == b.start || dest == b.end {
	//dest = nil
	//}
	v.succs = append(v.succs, dest)
	w.preds = append(w.preds, src)
	return b
}

// block returns a block for the given statement,
// creating one and inserting it into cfg if it doesn't already exist.
func (b *builder) block(s ast.Stmt) *block {
	bl, ok := b.bMap[s]
	if !ok {
		bl = newBlock(s)
		b.bMap[s] = bl
	}
	return bl
}

// pushDefer will add a defer statement to the front of the
// defer statements to be handled at any return or at the end of the CFG.
//
// TODO defers in conditionals and fors = mindfreak
func (b *builder) pushDefer(d *ast.DeferStmt) *builder {
	if b.dHead == nil {
		b.dHead = d
		b.dTail = d
		b.flowTo(d, b.end)
	} else {
		b.flowTo(d, b.dHead)
		b.dHead = d
	}
	return b
}

// buildStmt will handle most types of statements,
// connecting the current statement to the next in
// the graph. It will also delegate any control structures
// that aren't so simple to the appropriate function to
// handle. Each buildXxx method will define its own edges.
func (b *builder) buildStmt(cur, next ast.Stmt) *builder {
	b.edges = nil // reset for each statement

	switch s := cur.(type) {
	case *ast.IfStmt:
		b.buildIf(s, next)
	case *ast.ForStmt, *ast.RangeStmt:
		b.buildFor(s, next)
	case *ast.SwitchStmt, *ast.SelectStmt, *ast.TypeSwitchStmt:
		b.buildSwitch(s, next)
	case *ast.BranchStmt:
		b.buildBranch(s)
	case *ast.LabeledStmt:
		b.flowTo(cur, s.Stmt).buildStmt(s.Stmt, next)
	case *ast.ReturnStmt:
		b.buildReturn(s)
	default:
		b.flowTo(cur, next)
	}
	return b.buildEdges(next) // connect all new edges from buildXxx
}

// buildReturn hooks up return to the defer stack if it
// exists, otherwise the end. No edges.
//
// TODO this makes assumption that control flow is only in function?
func (b *builder) buildReturn(s ast.Stmt) *builder {
	if b.dHead != nil {
		b.flowTo(s, b.dHead)
	} else {
		b.flowTo(s, b.end)
	}
	return b
}

// buildBranch builds goto, break and continue statements.
// fallthrough is handled in the switch builder.
// No edges. Adds breaks/continue to branches to be handle appropriately later.
func (b *builder) buildBranch(br *ast.BranchStmt) *builder {
	switch br.Tok {
	case token.GOTO:
		b.flowTo(br, br.Label.Obj.Decl.(ast.Stmt))
	case token.FALLTHROUGH: // these will get handled in traverseSwitch
	default: // break/continue
		b.branches = append(b.branches, br)
	}
	return b
}

// buildEdges makes current block edges to flow to given statement.
func (b *builder) buildEdges(next ast.Stmt) *builder {
	for _, e := range b.edges {
		b.flowTo(e, next)
	}
	return b
}

// buildIf builds an IfStmt. It sets b.edges to the two successors.
func (b *builder) buildIf(f *ast.IfStmt, next ast.Stmt) *builder {
	var cur ast.Stmt = f
	if f.Init != nil {
		b.flowTo(f, f.Init)
		cur = f.Init
	}

	// We have to keep track of all edges from each block we parse
	// within this method in order to return later.
	var edges []ast.Stmt

	// build then
	edges = append(edges, b.buildBlock(cur, f.Body.List).edges...)

	switch s := f.Else.(type) {
	case *ast.BlockStmt:
		//     if
		//    /  \
		// then   else
		edges = append(edges, b.buildBlock(cur, s.List).edges...)
	case *ast.IfStmt:
		//     if
		//    /  \
		// then   else if
		//        /   \
		//      then   ?
		b.flowTo(cur, s)
		edges = append(edges, b.buildIf(s, next).edges...)
	default:
		//     if
		//    / |
		// then |
		edges = append(edges, b.flowTo(f, next).edges...)
	}
	b.edges = edges
	return b
}

// buildFor builds a loop (ForStmt or RangeStmt). It requires
// traversing its body as well as hooking up any unlabeled
// branches or labeled branches meant for this statement.
// Returns the for as edge.
func (b *builder) buildFor(stmt ast.Stmt, next ast.Stmt) *builder {
	var post ast.Stmt // post statement in for loop

	switch stmt := stmt.(type) {
	case *ast.ForStmt:
		// e.g. for [ init ]; ; [post] {
		//        Body
		//      }
		//
		// flows [ init -> ] for -> body -> [ post -> ] for -> next
		if stmt.Init != nil {
			b.flowTo(stmt.Init, stmt)
		}

		b.buildBlock(stmt, stmt.Body.List)

		// all edges either flow back to for or post, so hook up edges here
		if stmt.Post != nil {
			post = stmt.Post
			b.buildEdges(stmt.Post).flowTo(stmt.Post, stmt)
		} else {
			b.buildEdges(stmt)
		}
	case *ast.RangeStmt:
		// e.g. for i, _ := range {
		//        Body
		//      }
		// flows same as ForStmt w/o init or post
		b.buildBlock(stmt, stmt.Body.List).buildEdges(stmt)
	}

	// ForStmt is only edge at this point, any breaks will be added next.
	b.edges = []ast.Stmt{stmt}

	// handle any branches; if no label or for me: handle and remove from branches.
	for i := 0; i < len(b.branches); i++ {
		br := b.branches[i].(*ast.BranchStmt)
		if br.Label == nil || br.Label.Obj.Decl.(*ast.LabeledStmt).Stmt == stmt {
			switch br.Tok {
			case token.CONTINUE:
				if post != nil {
					b.flowTo(br, post)
				} else {
					b.flowTo(br, stmt)
				}
			case token.BREAK:
				b.flowTo(br, next)
			}
			b.branches = append(b.branches[:i], b.branches[i+1:]...)
			i-- // removed in place, so go back to this i
		}
	}
	return b
}

// buildSwitch builds a multi-way branch statement, e.g.
// switch, type switch and select. Sets [# of case] edges.
func (b *builder) buildSwitch(sw, next ast.Stmt) *builder {
	// composition of expected statement sw:
	//
	//    switch: *ast.SwitchStmt || *ast.TypeSwitchStmt || *ast.SelectStmt
	//      Body.List: []*ast.CaseClause || []ast.CommClause
	//        clause: []ast.Stmt
	//
	// current behavior is:
	//        switch
	//      /   |    \
	//    case case [default]
	//      |   |     |
	//  []stmt []stmt []stmt
	//       \  |   /
	//         next

	var cur ast.Stmt = sw

	switch s := cur.(type) {
	case *ast.SwitchStmt:
		// i.e. switch [ x := 0; ] x { }
		if s.Init != nil {
			b.flowTo(s, s.Init)
			cur = s.Init
		}
	case *ast.TypeSwitchStmt:
		// i.e. switch [ x := 0; ] t := x.(type) { }
		if s.Init != nil {
			b.flowTo(s, s.Init)
			cur = s.Init
		}
		b.flowTo(cur, s.Assign)
		cur = s.Assign
	}

	defaultCase := false
	var cases []ast.Stmt // case 1:, case 2:, ...

	switch sw := sw.(type) {
	case *ast.TypeSwitchStmt:
		cases = sw.Body.List
	case *ast.SwitchStmt:
		cases = sw.Body.List
	case *ast.SelectStmt:
		cases = sw.Body.List
	}

	var edges []ast.Stmt // edge(s) of each case

	for i, clause := range cases {
		b.flowTo(cur, clause)

		var caseBody []ast.Stmt

		// both of the following cases are guaranteed in spec
		switch cl := clause.(type) {
		case *ast.CaseClause: // switch/type switch
			// i.e. case: [expr,expr,...]:
			if cl.List == nil {
				defaultCase = true
			}
			caseBody = cl.Body
		case *ast.CommClause: // select
			// i.e. case c <- chan:
			if cl.Comm == nil {
				defaultCase = true
			} else {
				b.flowTo(cl, cl.Comm)
				clause = cl.Comm
			}
			caseBody = cl.Body
		}

		if ft := fallThrough(caseBody); ft != nil {
			b.flowTo(ft, cases[i+1])
		}

		edges = append(edges, b.buildBlock(clause, caseBody).edges...)
	}
	b.edges = edges

	if !defaultCase {
		// if default case exists, then assume switch will flow there.
		// if no default, switch may never execute any case and therefore
		// control could go from switch to next statement in block.
		//
		// i.e.
		//  -- switch {
		//  |  case:
		//  |  }
		//  -->nextStmt
		b.flowTo(cur, next)
	}

	// handle any breaks that are unlabeled or for me
	for i := 0; i < len(b.branches); i++ {
		if br := b.branches[i].(*ast.BranchStmt); br.Tok == token.BREAK &&
			(br.Label == nil || br.Label.Obj.Decl.(*ast.LabeledStmt).Stmt == cur) {

			b.flowTo(br, next)
			b.branches = append(b.branches[:i], b.branches[i+1:]...)
			i-- // we removed in place, so go back to this index
		}
	}

	return b
}

// fallThrough returns the fallthrough stmt at the end of stmts, if any.
func fallThrough(stmts []ast.Stmt) *ast.BranchStmt {
	if len(stmts) < 1 {
		return nil
	}

	// fallthrough can only be last statement in clause (possibly labeled)
	ft := stmts[len(stmts)-1]

	// Recursively descend LabeledStmts.
	for {
		switch s := ft.(type) {
		case *ast.BranchStmt:
			if s.Tok == token.FALLTHROUGH {
				return s
			}
		case *ast.LabeledStmt:
			ft = s.Stmt
			continue
		}
		break
	}
	return nil
}

// nextInBlock returns the next statement in a given block
// based off of given index i. Will return nil if next is
// outside the bounds of the block and will skip any defer
// statements.
func (b *builder) nextInBlock(s []ast.Stmt, i int) (ast.Stmt, int) {
	i++
	if i >= len(s) {
		return nil, i
	}
	switch stmt := s[i].(type) {
	case *ast.DeferStmt:
		return b.pushDefer(stmt).nextInBlock(s, i)
	default:
		return stmt, i
	}
	return nil, i // unreachable?
}

// buildBlock will iterate over a slice of statements
// (re: *ast.BlockStmt, mostly) and then should only have
// the last statement's edges as the builder's edges upon return.
func (b *builder) buildBlock(owner ast.Stmt, block []ast.Stmt) *builder {
	if len(block) < 1 { // empty blocks happen
		b.edges = append(b.edges, owner)
		return b
	}

	cur, i := b.nextInBlock(block, -1) // first in block (skip defers)
	b.flowTo(owner, cur)

	for i < len(block) {
		cur = block[i]
		var next ast.Stmt
		next, i = b.nextInBlock(block, i) // increments i, skips defers

		b.buildStmt(cur, next) // magic
	}
	return b
}

// buildGenKill builds the GEN and KILL bitsets for each block in a builder's cfg.
// Used to compute reaching definitions and live variable analysis.
func (b *builder) buildGenKill() {
	okills := make(map[*ast.Object]*bitset.BitSet)

	for stmt, _ := range b.bMap {
		b.gen[stmt] = bitset.New(0)
		b.kill[stmt] = bitset.New(0)
		b.blocks = append(b.blocks, stmt)
	}

	// Iterate over all blocks twice, because a block may not know the entirety of what
	// it kills until all blocks have been iterated over.
	for i := 0; i < 2; i++ {
		for j, stmt := range b.blocks {
			j := uint(j)
			var exprs []ast.Expr
			// extract all "producing" statements; assignments
			switch stmt := stmt.(type) {
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
						break
					}
					if _, ok := okills[e.Obj]; !ok {
						okills[e.Obj] = bitset.New(0)
					}
					b.gen[stmt].Set(j)
					okills[e.Obj].Set(j)
					b.kill[stmt] = b.kill[stmt].Union(okills[e.Obj]).Difference(b.gen[stmt])
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
func (b *builder) buildReaching() {

	ins := make(map[ast.Stmt]*bitset.BitSet)
	outs := make(map[ast.Stmt]*bitset.BitSet)

	// OUT[ENTRY] = {};
	outs[b.start] = bitset.New(0)

	// mblocks will be all of the cfg blocks except ENTRY
	var mblocks []ast.Stmt

	// for(each basic block B other than ENTRY) OUT[B} = {};
	for i := 0; i < len(b.blocks); i++ {
		stmt := b.blocks[i]
		ins[stmt] = bitset.New(0)
		outs[stmt] = bitset.New(0)
		if stmt != b.start {
			mblocks = append(mblocks, stmt)
		}
	}

	// for(changes to any OUT occur)
	for change := true; change; { // best do-while impersonation I got
		change = false

		// for(each basic block B other than ENTRY) {
		for _, stmt := range mblocks {

			// IN[B] = Union(P a pred of B) OUT[P];
			for _, p := range b.bMap[stmt].preds {
				ins[stmt].InPlaceUnion(outs[p])
			}

			old := outs[stmt].Clone()

			// OUT[B] = gen[b] Union (IN[B] - kill[b]);
			outs[stmt] = b.gen[stmt].Union(ins[stmt].Difference(b.kill[stmt]))

			change = change || !old.Equal(outs[stmt])
		}
	}

	// map bits in IN and OUT back to corresponding statements
	for _, stmt := range b.blocks {
		block := b.bMap[stmt]
		for i, ok := uint(0), true; ok; i++ {
			if i, ok = ins[stmt].NextSet(i); ok {
				block.in = append(block.in, b.blocks[i])
			}
		}

		for i, ok := uint(0), true; ok; i++ {
			if i, ok = outs[stmt].NextSet(i); ok {
				block.out = append(block.out, b.blocks[i])
			}
		}
	}
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
func (b *builder) buildLiveVariable() {

	ins := make(map[ast.Stmt]*bitset.BitSet)
	outs := make(map[ast.Stmt]*bitset.BitSet)

	// IN[EXIT] = {};
	ins[b.end] = bitset.New(0)

	// mblocks will be all of the cfg blocks except ENTRY
	var mblocks []ast.Stmt

	// for(each basic block B other than ENTRY) OUT[B} = {};
	for i := 0; i < len(b.blocks); i++ {
		stmt := b.blocks[i]
		ins[stmt] = bitset.New(0)
		outs[stmt] = bitset.New(0)
		if stmt != b.end {
			mblocks = append(mblocks, stmt)
		}
	}

	// for(changes to any OUT occur)
	for change := true; change; { // best do-while impersonation I got
		change = false

		// for(each basic block B other than EXIT) {
		for _, stmt := range mblocks {

			// IN[B] = Union(P a pred of B) OUT[P];
			for _, s := range b.bMap[stmt].succs {
				outs[stmt].InPlaceUnion(ins[s])
			}

			old := ins[stmt].Clone()

			// OUT[B] = gen[b] Union (IN[B] - kill[b]);
			ins[stmt] = b.gen[stmt].Union(outs[stmt].Difference(b.kill[stmt]))

			change = change || !old.Equal(ins[stmt])
		}
	}

	// map bits in IN and OUT back to corresponding statements
	for _, stmt := range b.blocks {
		block := b.bMap[stmt]
		for i, ok := uint(0), true; ok; i++ {
			if i, ok = ins[stmt].NextSet(i); ok {
				block.liveIn = append(block.liveIn, b.blocks[i])
			}
		}

		for i, ok := uint(0), true; ok; i++ {
			if i, ok = outs[stmt].NextSet(i); ok {
				block.liveOut = append(block.liveOut, b.blocks[i])
			}
		}
	}
}
