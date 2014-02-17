// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cfg contains an implementation for constructing a
// statement level Control Flow Graph (CFG) for Go code.
// It is intended to construct control flow graphs from
// an abstract syntax tree, however, any list of statements will do.
// This is done by traversing a list of statements (likely
// from a block) in DFS manner and creating an adjacency list,
// implemented as a hash map of vertices, with no explicit
// edge objects. Adjacent vertices are stored as predecessors
// and successors separately for control flow information.
//
// TODO (reed) I should probably provide a better description of the algorithm,
// and I will once it is stable and has been heavily refactored.
//
// Example Usage:
//
//    package main
//
//    import (
//      "go/ast"
//      "go/parser"
//      "go/token"
//      "golang-refactoring.org/go-doctor/extras/cfg"
//    )
//
//    func main() {
//      src := `
//        package main
//
//        import "fmt"
//
//        func main() {
//          for {
//            if 1 > 0 {
//              fmt.Println("my computer works")
//            } else {
//              fmt.Println("something has gone terribly wrong")
//            }
//          }
//        }
//      `
//
//      fset := token.NewFileSet()
//      f, err := parser.ParseFile(fset, "", src, 0)
//      funcOne := f.Decls[0].(*ast.FuncDecl)
//      c := cfg.FuncCFG(funcOne)
//      ast.Inspect(f, func(n ast.Node) bool {
//        switch stmt := f.(type) {
//        case *ast.Stmt:
//          s := c.Succs(stmt)
//          p := c.Preds(stmt)
//          //do as you please
//        }
//      }
//    }
//
package cfg

import (
	"go/ast"
	"go/token"
)

// TODO builder design pattern in the works
// TODO DFS Visit function?
// TODO transitive closure function?
// TODO end/begin checker funcs? end/begin exported setter funcs?
// BUG(reed): defer functions are finicky, mainly due to how
// they are to be handled within conditionals/fors. The base
// case is taken care of... thought needs to be had for special cases.

// Vertices are represented inside of a map of statements
// to vertices, where the vertex is a representation of a
// statement with knowledge of its neighbors in the graph.
//
// start, end are the beginning and end nodes, and are not
// actually members of the underlying []ast.Stmt that were
// given to construct the graph. They are useful for return
// and defer statement handling as well as declaring a singular
// entry (and exit) node.
type CFG struct {
	vMap       map[ast.Stmt]*vertex
	start, end *ast.BadStmt
}

// MakeCFG will create a new Control Flow Graph (CFG) from a given
// list of statements. Generally, these statements are assumed to
// have come from a *ast.BlockStmt (and eventually this may be entry)
// so as to have some sense of "flow" but any list of statements will do.
// They will be iterated over in depth first manner.
//
// TODO (reed) read more java design pattern books for builder entry
// TODO (reed) don't hook up defers to end unless we have a return or end of func?
func MakeCFG(s []ast.Stmt) *CFG {
	return newCFGBuilder().build(s)
}

// FuncCFG is a convenience wrapper function for creating a CFG
// from a given function. This may be entirely unnecessary but in
// my attempt to create a nice interface this popped out.
//
// Also taking suggestions for other useful entry points.
func FuncCFG(f *ast.FuncDecl) *CFG {
	return MakeCFG(f.Body.List)
}

//Preds returns a slice of all immediate predecessors to the given statement
//TODO if map would be more convenient, do speak
func (c *CFG) Preds(s ast.Stmt) []ast.Stmt {
	//TODO remove START/END? return err if no stmt?
	preds := make([]ast.Stmt, 0)
	if v, ok := c.vMap[s]; ok {
		for k, _ := range v.preds {
			if k != c.end || k != c.start {
				preds = append(preds, k)
			}
		}
	}
	return preds
}

//Succs returns a slice of all immediate successors to the given statement
//TODO if map would be more convenient, do speak
func (c *CFG) Succs(s ast.Stmt) []ast.Stmt {
	//TODO remove START/END? return err if no stmt?
	succs := make([]ast.Stmt, 0)
	if v, ok := c.vMap[s]; ok {
		for k, _ := range v.succs {
			if k != c.end || k != c.start {
				succs = append(succs, k)
			}
		}
	}
	return succs
}

// Vertices are represented inside of a map of statements
// to vertices, where the vertex is a representation of a
// statement with knowledge of its neighbors in the graph.
//
// edges are all of the current leaf nodes that need to be
// hooked up in some manner at the next build iteration.
// i.e. if/else has 2 (typically) leaf nodes, most statements
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
type builder struct {
	vMap            map[ast.Stmt]*vertex
	edges, branches []ast.Stmt
	start, end      *ast.BadStmt
	dHead, dTail    *ast.DeferStmt
}

//create a more reasonable zero value builder; ready to go
func newCFGBuilder() *builder {
	return &builder{make(map[ast.Stmt]*vertex),
		make([]ast.Stmt, 0), make([]ast.Stmt, 0),
		&ast.BadStmt{}, &ast.BadStmt{},
		nil, nil,
	}
}

// workhorse. runs buildBlock and then appropriately
// hooks up the defers (if present) and end,
// returning only the data that we need in a CFG
func (b *builder) build(s []ast.Stmt) *CFG {
	b.buildBlock(b.start, s)
	if b.dHead != nil {
		b.flowTo(b.dTail, b.end).buildEdges(b.dHead)
	} else {
		b.buildEdges(b.end)
	}
	return &CFG{b.vMap, b.start, b.end}
}

// vertex maps directly to an ast.Stmt, with
// predecessors and successors stored separately
// for later convenience
//
//TODO see listed kinks for preds/succs
type vertex struct {
	stmt  ast.Stmt
	preds map[ast.Stmt]*vertex //not sure if []*Vertex would be as useful?
	succs map[ast.Stmt]*vertex //map[ast.Stmt]*Vertex would disallow double entries/convenience?
}

// about the zero value thing...
func makeVertex(s ast.Stmt) *vertex {
	return &vertex{s, make(map[ast.Stmt]*vertex), make(map[ast.Stmt]*vertex, 0)}
}

// Will access or create vertex for given statements
// and then flow from src to dest, appropriately
// adding to successors/predecessors, as well.
//
// If dest == nil: add src as edge to handle at higher level.
func (b *builder) flowTo(src, dest ast.Stmt) *builder {
	if dest == nil {
		b.edges = append(b.edges, src)
		return b
	}
	v := b.getVertex(src)
	w := b.getVertex(dest)
	v.succs[dest] = w
	w.preds[src] = v
	return b
}

// If DNE, creates vertex for statement and
// inserts into CFG's vMap.
//
// Returns vertex for given statement
func (b *builder) getVertex(s ast.Stmt) (v *vertex) {
	v, ok := b.vMap[s]
	if !ok {
		v = makeVertex(s)
		b.vMap[s] = v
	}
	return
}

// Will add a defer statement to the "stack"
// of defer statements to be handled at any return
// or at the end of
//
//
//TODO defers in conditionals = mindfreak
func (b *builder) pushDefer(d *ast.DeferStmt) *builder {
	switch {
	case b.dHead == nil:
		b.dHead, b.dTail = d, d
		b.flowTo(d, b.end)
	default:
		b.flowTo(d, b.dHead)
		b.dHead = d
	}
	return b
}

// For muxing, and handling of most simple stmts.
// Each builderXxx method will define its own "edges"
// if we're at the edge of a block, with the default
// being set to 'cur'.
func (b *builder) buildStmt(cur, next ast.Stmt) *builder {
	b.edges = nil //reset for each statement

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
	case *ast.DeferStmt, nil: //TODO necessary?
	default:
		b.flowTo(cur, next)
	}
	return b.buildEdges(next)
}

// Hooks up return to the defer stack if it
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

// Builds goto, break and continue. fallthrough is handled
// in the switch builder. No edges. Adds breaks/continue
// to branches to be handle appropriately later.
func (b *builder) buildBranch(br *ast.BranchStmt) *builder {
	switch br.Tok {
	case token.GOTO:
		//TODO jury still out on whether to flow to label or its statement
		b.flowTo(br, br.Label.Obj.Decl.(*ast.LabeledStmt).Stmt)
	case token.FALLTHROUGH: //these will get handled in traverseSwitch
	default: //break/continue
		b.branches = append(b.branches, br)
	}
	return b
}

// Just got sick of writing this for loop
func (b *builder) buildEdges(next ast.Stmt) *builder {
	for _, e := range b.edges {
		b.flowTo(e, next)
	}
	return b
}

// Build if statement appropriately, returning multiple
// edges.
func (b *builder) buildIf(f *ast.IfStmt, next ast.Stmt) *builder {
	var cur ast.Stmt = f
	if f.Init != nil {
		b.flowTo(f, f.Init)
		cur = f.Init
	}

	edges := make([]ast.Stmt, 0)
	edges = append(edges, b.buildBlock(cur, f.Body.List).edges...)

	switch s := f.Else.(type) {
	case *ast.BlockStmt:
		//     if
		//    /  \
		//then    else
		edges = append(edges, b.buildBlock(cur, s.List).edges...)
	case *ast.IfStmt:
		//     if
		//    /  \
		//then    else if
		//        /   \
		//      then   ?
		b.flowTo(cur, s)
		edges = append(edges, b.buildIf(s, next).edges...)
	default:
		//    if
		//   / |
		//then |
		edges = append(edges, b.flowTo(f, next).edges...)
	}
	b.edges = edges
	return b
}

// Build for and range. Requires traversing block as well
// as hooking up any unlabeled branches or branches meant
// for this statement. Returns for as edge
func (b *builder) buildFor(stmt ast.Stmt, next ast.Stmt) *builder {

	var post ast.Stmt //post condition in for loop

	switch s := stmt.(type) {
	// for i := 0; i < len(list); i++ {
	// for [ init ]; ; [post] {
	//  Body
	// }
	case *ast.ForStmt:
		if s.Init != nil {
			//TODO hard to follow?
			b.flowTo(s.Init, s) //currently assign -> for -> body -> post -> for
		}

		b.buildBlock(stmt, s.Body.List)

		if s.Post != nil {
			post = s.Post
			b.buildEdges(s.Post).flowTo(s.Post, stmt)
		} else {
			b.buildEdges(stmt)
		}
	case *ast.RangeStmt:
		// for i, _ := range {
		//  Body
		// }
		b.buildBlock(s, s.Body.List)
	}

	b.edges = []ast.Stmt{stmt}

	//handle appropriate branches
	for j := 0; j < len(b.branches); {
		br := b.branches[j].(*ast.BranchStmt)
		//if nil label or for me: handle and remove from branches
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
			b.branches = append(b.branches[:j], b.branches[j+1:]...) //delete
		} else {
			j++
		}
	}
	return b
}

// Build switch/type switch/select... any name that means all three?
// Thanks to the above, this is a mess of type switches.
// Nevertheless, switch, type switch and select are all handled
// very similarly for control flow.
//
// TODO this is 120 lines w/o Buzz... taking ideas
func (b *builder) buildSwitch(sw, next ast.Stmt) *builder {
	//
	//                      _._
	//                      ||||
	//       ___           _||||
	//    .-'___`-.        ||  |
	//   ' .'_ _'. '.      \   /
	//  | (| b d |) |       |~~\
	//  |  |  '  |  |       |  `\
	// ,|  | `-' |  |,      :-.__\        ,
	///.|  /\___/\  |.\""''-/    )-------'|
	///   '-._____.-'   \   /'-._/        |
	//|_    .---. ===  |_.'\    /--------.|
	//|\_\ _ \=v=/  _   | |  \ /         '
	//| \_\_\ ~~~  (_)  | |  .'
	//|`'--.__.^.__.--'`|'"'`
	//\                 /
	// `,..---'"'---..,'
	//   :--..___..--:    Type assertions...
	//    \         /
	//    |`.     .'|       Type assertions everywhere...

	//
	// switch: *ast.SwitchStmt
	//  Body.List: []*ast.CaseClause
	//    clause: []ast.Stmt
	var cur ast.Stmt = sw

	switch s := cur.(type) {
	case *ast.SwitchStmt:
		//switch [ x := 0; ] x { }
		if s.Init != nil {
			b.flowTo(s, s.Init)
			cur = s.Init
		}
	case *ast.TypeSwitchStmt:
		//switch [ x := 0; ] t := x.(type) { }
		if s.Init != nil {
			b.flowTo(s, s.Init)
			cur = s.Init
		}
		b.flowTo(cur, s.Assign)
		cur = s.Assign
	}

	defaultCase := false
	var cases []ast.Stmt //case 1:, case 2:, ...

	switch s := sw.(type) { //guess these don't play so nice...
	case *ast.TypeSwitchStmt:
		cases = s.Body.List
	case *ast.SwitchStmt:
		cases = s.Body.List
	case *ast.SelectStmt:
		cases = s.Body.List
	}

	//TODO do each of these flow to the next case, then default (or next) ?
	//
	//current behavior is:
	//        switch
	//      /   |    \
	//    case case [default]
	//      |   |     |
	//  []stmt []stmt []stmt
	//       \  |   /
	//         next
	for i, clause := range cases {
		b.flowTo(cur, clause)

		var caseBody []ast.Stmt

		//both following are guaranteed in spec
		switch cl := clause.(type) {
		case *ast.CaseClause: //switch/type switch
			//case: [expr,expr,...]:
			if cl.List == nil {
				defaultCase = true
			}
			caseBody = cl.Body
		case *ast.CommClause: //select
			//case c <- chan:
			if cl.Comm == nil {
				defaultCase = true
			} else {
				b.flowTo(cl, cl.Comm)
				clause = cl.Comm
			}
			caseBody = cl.Body
		}

		//fallthrough can only be last statement in clause (possibly labeled)
		if len(caseBody) > 0 {
			ft := caseBody[len(caseBody)-1]
		lbl: // b/c labels can be labeled... we need to go deeper
			switch last := ft.(type) {
			case *ast.BranchStmt:
				if last.Tok == token.FALLTHROUGH {
					b.flowTo(ft, cases[i+1])
				}
			case *ast.LabeledStmt:
				ft = last.Stmt
				goto lbl
			}
		}

		b.buildBlock(clause, caseBody).buildEdges(next) //build case block's edges
		//TODO (reed) I'm confused on why this works... not good. act like if?
	}

	if !defaultCase {
		// if default case exists, then assume switch will flow there.
		// if no default, switch may never execute any case and therefore
		// control could go from switch to next statement in block.
		//
		// e.g.
		//  -- switch {
		//  |  case:
		//  |  }
		//  -->nextStmt
		b.flowTo(cur, next)
	}

	//handle any breaks that are unlabeled or for me
	for j := 0; j < len(b.branches); {
		//if nil label or for me: handle and remove from branches
		if br := b.branches[j].(*ast.BranchStmt); br.Tok == token.BREAK &&
			(br.Label == nil || br.Label.Obj.Decl.(*ast.LabeledStmt).Stmt == cur) {
			b.flowTo(br, next)
			b.branches = append(b.branches[:j], b.branches[j+1:]...) //remove
		} else {
			j++
		}
	}

	return b
}

// Convenience func to skip defers and return nil
// if next would be OOB. Have to handle defers here
// to prevent them being flowed to until end.
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
	return nil, i //unreachable?
}

// Pretty popular guy, will iterate over a slice of statements
// (re: *ast.BlockStmt, but switch messed that all up) and then
// should only have the last statement's edges as the builder's
// edges upon return.
func (b *builder) buildBlock(owner ast.Stmt, block []ast.Stmt) *builder {
	if len(block) < 1 { //empty blocks happen
		b.edges = append(b.edges, owner)
		return b
	}

	cur, i := b.nextInBlock(block, -1) //first in block (skip defers)
	b.flowTo(owner, cur)

	for i < len(block) {
		cur = block[i]
		var next ast.Stmt
		next, i = b.nextInBlock(block, i) //increments i, skips defers

		b.buildStmt(cur, next) //magic
	}
	return b
}
