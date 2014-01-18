// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains an implementation for constructing a
// statement level Control Flow Graph (CFG) by traversing
// a list of statements and creating an adjacency list,
// implemented as a hash map of vertices, with no explicit
// edge objects. Adjacent vertices are stored as predecessors
// and successors separately for control flow information.

// TODO (reed) I should probably provide a better description of the algorithm,
// and I will once it is stable and has been heavily refactored.

// Exported Functions:
//    FuncCFG(*ast.FuncDecl) *CFG
//    MakeCFG([]ast.Stmt) *CFG
//    cfg.Succs() []ast.Stmt
//    cfg.Preds() []ast.Stmt

// Example:
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

// TODO dear god please bestow reed with a reasonable refactoring
//    ...Builder... immutable style?
// TODO DFS Visit function?
// TODO transitive closure function?

// Vertices are represented inside of a map of statements
// to vertices, where the vertex is a representation of a
// statement with knowledge of its neighbors in the graph.
//
// start, end are the beginning and end nodes, and are not
// actually members of the underlying []ast.Stmt that were
// given to construct the graph. They are useful for return
// and defer statement handling as well as declaring a singular
// entry (and exit) node. TODO need to strip from returned preds/succs
//
// dHead, dTail are analagous to the head and tail of the defer
// stack that will be called upon return of control, directly
// after any return statements and before the actual end node.
type CFG struct {
	vMap         map[ast.Stmt]*vertex
	start, end   *ast.BadStmt
	dHead, dTail *ast.DeferStmt
}

//convenience func for interfacing //TODO useful?
//
// Given a function declaration will create a
// CFG from the body of the function
func FuncCFG(f *ast.FuncDecl) *CFG {
	return MakeCFG(f.Body.List)
}

// Main entry point for creating a CFG from a list of statements.
// Will iterate over in depth first manner. Most likely useful from
// an ast node that has a Body that is []ast.Stmt, but gives flexibility
// to allow user to create graph from any list of statements.
func MakeCFG(s []ast.Stmt) *CFG {
	cfg := &CFG{make(map[ast.Stmt]*vertex),
		&ast.BadStmt{}, &ast.BadStmt{},
		nil, nil,
	}
	edges, _ := cfg.traverseBody(cfg.start, s)
	for _, e := range edges {
		if cfg.dHead != nil { //see return behavior
			cfg.flowTo(e, cfg.dHead)
			e = cfg.dTail
		}
		cfg.flowTo(e, cfg.end)
	}
	return cfg
}

//Returns a slice of all immediate predecessors to the given statement
func (c *CFG) Predecessors(s ast.Stmt) []ast.Stmt {
	//TODO remove START/END?
	return c.getVertex(s).preds
}

//Returns a slice of all immediate successors to the given statement
func (c *CFG) Successors(s ast.Stmt) []ast.Stmt {
	//TODO remove START/END?
	return c.getVertex(s).succs
}

//TODO see listed kinks for preds/succs
type vertex struct {
	stmt  ast.Stmt
	preds []ast.Stmt //not sure if []*Vertex would be as useful?
	succs []ast.Stmt //map[ast.Stmt]*Vertex would disallow double entries
}

func makeVertex(s ast.Stmt) *vertex {
	return &vertex{s, make([]ast.Stmt, 0), make([]ast.Stmt, 0)}
}

//if next == nil, return src to add to edges... this is really hard to follow in practice
func (c *CFG) flowTo(src, dest ast.Stmt) ast.Stmt {
	if dest == nil {
		return src
	}
	v := c.getVertex(src)
	w := c.getVertex(dest)
	v.succs = append(v.succs, dest)
	w.preds = append(w.preds, src)
	return nil
}

//TODO defers in conditionals = mindfreak
func (c *CFG) pushDefer(d *ast.DeferStmt) {
	switch {
	case c.dHead == nil:
		c.dHead, c.dTail = d, d
		c.flowTo(d, c.end)
	default:
		c.flowTo(d, c.dHead)
		c.dHead = d
	}
}

// If DNE, creates vertex for statement and
// inserts into CFG's vMap.
//
// Returns vertex for given statement
func (c *CFG) getVertex(s ast.Stmt) (v *vertex) {
	v, ok := c.vMap[s]
	if !ok {
		v = makeVertex(s)
		c.vMap[s] = v
	}
	return
}

//for muxing
//TODO 1 caller... maybe get rid of this altogether and then in its stead
//handle each case, e.g. for/if/switch/etc. by giving it the next in block
//(or nil) to "flow" to.
//
// Benefits: would clean up traverseBlock significantly.
// Cons:  the joys of breaking code
//        the edges still need to get returned (or do they?)
func (c *CFG) traverseStmt(stmt ast.Stmt) ([]ast.Stmt, []ast.Stmt) {
	if stmt == nil {
		return nil, nil
	}
	switch s := stmt.(type) {
	case *ast.IfStmt:
		return c.traverseIf(s)
	case *ast.ForStmt:
		return c.traverseFor(s)
	case *ast.BranchStmt, *ast.DeferStmt, *ast.ReturnStmt:
		return nil, nil //TODO this is dumb, but currently handled in traverseBlock()
	case *ast.SwitchStmt, *ast.SelectStmt, *ast.TypeSwitchStmt:
		return c.traverseSwitch(s)
	case *ast.RangeStmt:
		return c.traverseBody(s, s.Body.List)
	default:
		return []ast.Stmt{stmt}, nil
	}
	return nil, nil
}

//return all leaf nodes and branches
func (c *CFG) traverseIf(f *ast.IfStmt) ([]ast.Stmt, []ast.Stmt) {
	var cur ast.Stmt = f
	if f.Init != nil {
		c.flowTo(f, f.Init)
		cur = f.Init
	}

	leaves, branches := c.traverseBody(cur, f.Body.List)
	switch s := f.Else.(type) {
	case *ast.BlockStmt:
		//     if
		//    /  \
		//then    else
		l, b := c.traverseBody(cur, s.List)
		leaves, branches = append(leaves, l...), append(branches, b...)
	case *ast.IfStmt:
		//     if
		//    /  \
		//then    else if
		//        /   \
		//      then   ?
		c.flowTo(cur, s)
		l, b := c.traverseIf(s)
		leaves, branches = append(leaves, l...), append(branches, b...)
	default:
		//    if
		//   / |
		//then |
		leaves = append(leaves, f)
	}
	return leaves, branches
}

//return all leaf nodes...
func (c *CFG) traverseFor(s *ast.ForStmt) ([]ast.Stmt, []ast.Stmt) {
	var cur ast.Stmt = s
	if s.Init != nil {
		c.flowTo(s, s.Init)
		cur = s.Init
	}
	edges, branches := c.traverseBody(cur, s.Body.List)
	if s.Post != nil {
		for _, e := range edges {
			c.flowTo(e, s.Post)
		}
		return []ast.Stmt{s.Post}, branches
	}
	return edges, branches
}

//switch or type switch
func (c *CFG) traverseSwitch(sw ast.Stmt) ([]ast.Stmt, []ast.Stmt) {
	var cur ast.Stmt = sw
	switch s := cur.(type) {
	case *ast.SwitchStmt:
		if s.Init != nil {
			c.flowTo(s, s.Init)
			cur = s.Init
		}
	case *ast.TypeSwitchStmt:
		if s.Init != nil {
			c.flowTo(s, s.Init)
			cur = s.Init
		}
		c.flowTo(cur, s.Assign)
		cur = s.Assign
	}
	//
	//        switch
	//      /   |    \
	//    case case default
	//      |
	//    []stmt ...
	//
	// and ofc, only return leaves
	//
	// switch: *ast.SwitchStmt
	//  Body.List: []*ast.CaseClause
	//    clause: []ast.Stmt

	leaves, branches := make([]ast.Stmt, 0), make([]ast.Stmt, 0)
	defaultCase := false
	var cases []ast.Stmt

	//TODO these aren't so nice in practice?
	switch s := sw.(type) {
	case *ast.TypeSwitchStmt:
		cases = s.Body.List
	case *ast.SwitchStmt:
		cases = s.Body.List
	case *ast.SelectStmt:
		cases = s.Body.List
	}

	for i, clause := range cases {
		c.flowTo(cur, clause)

		var caseBody []ast.Stmt
		switch cl := clause.(type) {
		case *ast.CaseClause:
			if cl.List == nil {
				defaultCase = true
			}
			caseBody = cl.Body
		case *ast.CommClause:
			if cl.Comm == nil {
				defaultCase = true
			} else {
				c.flowTo(cl, cl.Comm)
				clause = cl.Comm
			}
			caseBody = cl.Body
		}
		l, b := c.traverseBody(clause, caseBody)
		leaves, branches = append(leaves, l...), append(branches, b...)

		//fallthrough can only be last statement in clause (also labeled)
		ft := caseBody[len(caseBody)-1]
	lbl: //b/c labels can be labeled... we need to go deeper
		switch last := ft.(type) {
		case *ast.BranchStmt:
			if last.Tok == token.FALLTHROUGH {
				nxt, _ := c.nextInBlock(cases, i) //return index not relevant with []clause
				l = append(l, c.flowTo(ft, nxt))
			}
		case *ast.LabeledStmt:
			ft = last.Stmt
			goto lbl
		}
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
		leaves = append(leaves, cur)
	}

	return leaves, branches
}

func (c *CFG) nextInBlock(b []ast.Stmt, i int) (ast.Stmt, int) {
	i++
	if i >= len(b) {
		return nil, i
	}
	switch s := b[i].(type) {
	case *ast.DeferStmt:
		c.pushDefer(s)
		return c.nextInBlock(b, i)
	default:
		return s, i
	}
	return nil, i //unreachable?
}

//return leaves, branches
//
//TODO sweet mother of $@#! clean this `god` function pattern up

//switch from block to []ast.Stmt
func (c *CFG) traverseBody(owner ast.Stmt, block []ast.Stmt) ([]ast.Stmt, []ast.Stmt) {
	if len(block) < 1 { //empty blocks happen
		return nil, nil
	}

	//TODO maybe consider making an object of these kinds of items?
	edges := make([]ast.Stmt, 0)    //reset at each stmt in block
	branches := make([]ast.Stmt, 0) //all ast.BranchStmt break/continue, don't discard at each block stmt

	cur, i := c.nextInBlock(block, -1) //first in block (skip defers)
	c.flowTo(owner, cur)

	//TODO Think long and hard about this...
	//If I just generate edges from statement in order to hook them up at the next thing,
	//do I actually need to hook them up to them next thing if they'll be returned as edges
	//on the last iteration or hooked up in the subsequent statement in a block?
	for i < len(block) {
		cur = block[i]
		var next ast.Stmt
		next, i = c.nextInBlock(block, i) //increments i

	withlabel:
		var brs []ast.Stmt //TODO potentially more concise way to do these 3
		edges, brs = c.traverseStmt(cur)
		branches = append(branches, brs...)

		switch s := cur.(type) {
		default:
			//add this to next in block
			c.flowTo(cur, next)
		case *ast.DeferStmt:
			//don't want to hook this up to anything
			continue
		case *ast.ReturnStmt:
			if c.dHead != nil {
				c.flowTo(s, c.dHead)
			} else {
				c.flowTo(s, c.end)
			}
			edges = nil
		case *ast.IfStmt:
			//for each edge of this, flow to next in block
			for _, k := range edges {
				c.flowTo(k, next)
			}
		case *ast.SwitchStmt, *ast.SelectStmt:
			//for each edge of this, flow to next in block
			for _, k := range edges {
				c.flowTo(k, next)
			}

			//handle any breaks that are unlabeled or for me
			for j := 0; j < len(branches); {
				//if nil label or for me: handle and remove from branches
				if b := branches[j].(*ast.BranchStmt); b.Tok == token.BREAK &&
					(b.Label == nil || b.Label.Obj.Decl.(*ast.LabeledStmt).Stmt == cur) {
					edges = append(edges, c.flowTo(b, next))
					branches = append(branches[:j], branches[j+1:]...)
				} else {
					j++
				}
			}
		case *ast.LabeledStmt:
			//Currently every successor from the statement of the labeled statement
			//will flow to the embedded statement... this is simply a design decision.
			//e.g.
			//    stmt
			//     |
			//    hello:
			//      \
			//       if
			//all branches will flow to the if, but the label is still flowed to [in block]
			//
			//Either way will work fine and neither appears immediately more convenient.
			//Open for discussion.
			c.flowTo(cur, s.Stmt)
			cur = s.Stmt
			goto withlabel
		case *ast.ForStmt, *ast.RangeStmt:
			//edges of for to next in block and for
			for _, k := range edges {
				c.flowTo(k, cur)
			}

			//edge of for to next, add FOR to edges if next == nil
			edges = append(edges, c.flowTo(cur, next))
			//rationale:
			//if flowto(this, next) isn't possible:
			//  return FOR as edge to next level as edge to hook up
			//
			//e.g.
			//  for {
			//  ^
			//   \
			//    for {
			//    }
			//  }

			for j := 0; j < len(branches); {
				b := branches[j].(*ast.BranchStmt)
				//if nil label or for me: handle and remove from branches
				if b.Label == nil || b.Label.Obj.Decl.(*ast.LabeledStmt).Stmt == cur {
					switch b.Tok {
					case token.CONTINUE:
						//TODO christ this became a mess
						// have to hook up continue to Post stmt if one
						// starting to think I could handle branches in traverseFor()?
						switch fcheck := s.(type) {
						case *ast.ForStmt:
							if fcheck.Post != nil {
								c.flowTo(b, fcheck.Post)
							} else {
								c.flowTo(b, cur)
							}
						case *ast.RangeStmt:
							c.flowTo(b, cur)
						}
						branches = append(branches[:j], branches[j+1:]...)
					case token.BREAK:
						edges = append(edges, c.flowTo(b, next)) //see above `rationale`
						branches = append(branches[:j], branches[j+1:]...)
					default: //uh, fallthrough?
						j++
					}
				} else {
					j++
				}
			}
		case *ast.BranchStmt:
			switch s.Tok {
			case token.GOTO:
				//TODO jury still out on whether to flow to label or its statement
				c.flowTo(s, s.Label.Obj.Decl.(*ast.LabeledStmt).Stmt)
			case token.FALLTHROUGH: //these will get handled in traverseSwitch
			default: //break/continue
				branches = append(branches, s)
			}
		}
	}
	return edges, branches
}
