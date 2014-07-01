// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dataflow

//something something

import (
	"go/ast"
	"go/token"
	"testing"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"

	"golang-refactoring.org/go-doctor/analysis/cfg"
)

const (
	START = 0
	END   = 100000000 //if there's this many statements, may god have mercy on your soul
)

func TestLiveTypeSwitch(t *testing.T) {
	c := getWrapper(t, `
  package main

  func main() {
    var x interface{} = 1.2 // 1
    switch i := x.(type) {  // 2 (switch), 3 (assignment)
    case int:               // 4
      fooi(i)               // 5
    }
  }
  func fooi(n int) {}
  func foof(n float64) {}`)

	c.expectLive(t, START)
	c.expectLive(t, 1, "x")
	c.expectLive(t, 2, "x", "i")
	c.expectLive(t, 3, "i")
	c.expectLive(t, 4, "i")
	c.expectLive(t, 5)
	c.expectLive(t, END)
}

func TestLiveLabeledLoopAndSwitch(t *testing.T) {
	c := getWrapper(t, `
  package main

  func main() {
    y:=5           // 1
    foo(y)         // 2
ABC:               // 3
    for {          // 4
      x := 1       // 5
      switch {     // 6
      case x > 0:  // 7
        foo(0)     // 8
        break ABC  // 9
      case x == 1: // 10
        foo(x)     // 11
      default:     // 12
        foo(2)     // 13
      }
    }
  }
  func foo(n int) {}`)

	c.expectLive(t, START)
	c.expectLive(t, 1, "y")
	c.expectLive(t, 2)
	c.expectLive(t, 3)
	c.expectLive(t, 4)
	c.expectLive(t, 5, "x")
	c.expectLive(t, 6, "x")
	c.expectLive(t, 7)
	c.expectLive(t, 8)
	c.expectLive(t, 9)
	c.expectLive(t, 10, "x")
	c.expectLive(t, 11)
	c.expectLive(t, 12)
	c.expectLive(t, 13)
	c.expectLive(t, END)
}

func TestLiveLabeledLoop(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo() int {
    a := 1      //1
loop:           //2
    for _, i := range []int{1,2} { //3
      b := i    //4
      a += b    //5
    }
    return a    //6
    goto loop   //7
    //END
  }`)

	c.expectLive(t, START)
	c.expectLive(t, 1, "a")
	c.expectLive(t, 2, "a")
	c.expectLive(t, 3, "a", "i")
	c.expectLive(t, 4, "a", "b")
	c.expectLive(t, 5, "a")
	c.expectLive(t, 6)
	c.expectLive(t, 7, "a")
	c.expectLive(t, END)
}

func TestExprStuff(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int, nums []int) int {
    //START
    a := c      //1
    var b int   //2
    b += 1      //3
    c, a = a, c //4
    b = a       //5
    for a, c = range nums { //6
      b += a    //7
    }
    a, c = c, a //8
    c = b       //9
    b++         //10
    return a    //11
    //END
  }`)

	c.expectDefs(t, START, 2, "a", "b")
	c.expectUses(t, START, 2, "c")

	c.expectReaching(t, START)
	c.expectReaching(t, 2, 1)
	c.expectReaching(t, 4, 3, 1)
	c.expectReaching(t, 6, 6, 4, 5, 7)
	c.expectReaching(t, 7, 7, 6, 5)
	c.expectReaching(t, 8, 7, 6, 5)
	c.expectReaching(t, 9, 8, 7, 5)

	c.expectLive(t, START, "c", "nums")
	c.expectLive(t, 1, "a", "c", "nums")
	c.expectLive(t, 2, "a", "b", "c", "nums")
	c.expectLive(t, 3, "a", "c", "nums")
	c.expectLive(t, 4, "a", "nums")
	c.expectLive(t, 5, "b", "nums")
	c.expectLive(t, 6, "a", "b", "c", "nums")
	c.expectLive(t, 7, "b", "nums")
	c.expectLive(t, 8, "a", "b")
	c.expectLive(t, 9, "a", "b")
	c.expectLive(t, 10, "a")
	c.expectLive(t, END)

	//c.printAST()
}

func TestIndexExpr(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int, nums []int) int {
    //START
    nMap := make(map[int]int) // 1
    for i, n := range nums { // 2
      if nums[i] == n { // 3
        nMap[n] = i // 4
      }
    }
    print(3) // 5
    return c // 6
    //END
  }`)

	c.expectLive(t, START, "c", "nums")
	c.expectLive(t, 1, "c", "nums", "nMap")
	c.expectLive(t, 2, "c", "nums", "nMap", "i", "n")
	c.expectLive(t, 3, "c", "nums", "nMap", "i", "n")
	c.expectLive(t, 4, "c", "nums", "nMap")
	c.expectLive(t, 5, "c")
	c.expectLive(t, END)
}

func TestExractFuncEx(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo() {
    a := 1
    b := a - 1  //2
    c := 1      //3

    // BEGIN EXTRACT
    for a < b { //4
      a += b    //5
    }
    x := a + b  //6
    // END EXTRACT
    
    z := x + c  //7
    _ = z
  }`)

	// only analyzes LIVE[OUT] for now in expectXxx method...
	// but of criticial importance, worst case for EXTRACT FUNC:
	c.expectLive(t, 3, "a", "b", "c")
	c.expectLive(t, 6, "x", "c")

	// So in extracted function, "c" never gets used so we don't need to
	// pass it as a parameter nor return it since the value never changes
	// within our extracted function.
	// Yet, it's still live for the duration of the extracted function.
	//
	//    current ideas:
	//      B = BEGIN EXTRACT
	//      E = END EXTRACT
	//
	//      PARAMS[EXTRACTED] = LIVE[IN][B] ∩ USE[EXTRACTED]
	//      RETURN[EXTRACTED] = LIVE[OUT][E] ∩ DEF[EXTRACTED]
	//
	// extracted function should be (best case):
	//
	//  func bar(a, b int) int {
	//    for a < b {
	//      a += b
	//    }
	//    x := a + b
	//    return x // or "return a + b" if you really want to analyze
	//  }
	//
	// caller should then be:
	//
	//  func foo() {
	//    a := 3
	//    b := a - 1
	//    c := 1
	//    x := bar(a, b)
	//    z := x + c
	//  }
}

func TestLiveDefers(t *testing.T) {
	c := getWrapper(t, `
  package main

  import (
    "os"
    "fmt"
  )

  func foo() {
    // START
    f, err := os.Open("foo") // 1
    if err != nil { // 2
      fmt.Println(err) // 3
      os.Exit(2) // 4
    }

    defer f.Close() // 5
    f.Write([]byte("bar")) // 6

    fmt.Println("done writing") // 7
    // END
  }`)

	c.expectLive(t, 1, "f", "err")
	c.expectLive(t, 7, "f")
	c.expectLive(t, END)
}

func TestFuncLit(t *testing.T) {
	c := getWrapper(t, `
  package main

  import (
    "fmt"
  )

  func foo() {
    // START
    f := func() { // 1
      fmt.Println("foo") // 2
    }
    for _ = range []int{1, 2, 3} { // 3
      f() // 4
    }

    fmt.Println("bar") // 5
    // END
  }`)

	c.expectLive(t, 1, "f")
	c.expectLive(t, 4, "f")
	c.expectLive(t, 5)
}

func BenchmarkReaching(b *testing.B) {
	src := `package main

  func foo(c int, nums []int) int {
    //START
    a := c      //1
    var b int   //2
    b += 1      //3
    c, a = a, c //4
    b = a       //5
    for a, c = range nums { //6
      b += a    //7
    }
    a, c = c, a //8
    c = b       //9
    b++         //10
    return a    //11
    //END
  }
    `
	var config loader.Config
	f, err := config.ParseFile("testing", src)
	if err != nil {
		b.Error(err.Error())
		b.FailNow()
	}

	pkg := loader.CreatePkg{"testing", []*ast.File{f}}
	config.CreatePkgs = []loader.CreatePkg{pkg}

	prog, err := config.Load()

	if err != nil {
		b.Error(err.Error())
		b.FailNow()
	}

	// create CFG and compute ReachingDefs
	for n := 0; n < b.N; n++ {
		cfg := cfg.FromFunc(f.Decls[0].(*ast.FuncDecl))
		ReachingDefs(cfg, prog.Created[0])
		LiveVars(cfg, prog.Created[0])
	}
}

func BenchmarkMain(b *testing.B) {
	var config loader.Config
	err := config.CreateFromFilenames("main", "../../main.go")
	if err != nil {
		b.Error(err.Error())
		b.FailNow()
	}

	prog, err := config.Load()

	if err != nil {
		b.Error(err.Error())
		b.FailNow()
	}

	// create CFG and compute ReachingDefs
	for n := 0; n < b.N; n++ {
		cfg := cfg.FromFunc(prog.Created[0].Files[0].Decls[7].(*ast.FuncDecl))
		ReachingDefs(cfg, prog.Created[0])
		LiveVars(cfg, prog.Created[0])
	}
}

// lo and behold how it's done -- caution: disgust may ensue
type CFGWrapper struct {
	cfg      *cfg.CFG
	prog     *loader.Program
	exp      map[int]ast.Stmt
	stmts    map[ast.Stmt]int
	objs     map[string]*types.Var
	objNames map[*types.Var]string
	fset     *token.FileSet
	f        *ast.File
}

// uses first function in given string to produce CFG
// w/ some other convenient fields for printing in test
// cases when need be...
func getWrapper(t *testing.T, str string) *CFGWrapper {
	var config loader.Config
	f, err := config.ParseFile("testing", str)
	if err != nil {
		t.Error(err.Error())
		t.FailNow()
		return nil
	}

	pkg := loader.CreatePkg{"testing", []*ast.File{f}}
	config.CreatePkgs = []loader.CreatePkg{pkg}

	prog, err := config.Load()

	if err != nil {
		t.Error(err.Error())
		t.FailNow()
		return nil
	}

	firstFunc, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok { // skip import decl if exists
		firstFunc = f.Decls[1].(*ast.FuncDecl) // panic here if no first func
	}
	cfg := cfg.FromFunc(firstFunc)
	v := make(map[int]ast.Stmt)
	stmts := make(map[ast.Stmt]int)
	objs := make(map[string]*types.Var)
	objNames := make(map[*types.Var]string)
	i := 1
	ast.Inspect(firstFunc, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if obj, ok := prog.Created[0].ObjectOf(x).(*types.Var); ok {
				objs[obj.Name()] = obj
				objNames[obj] = obj.Name()
			}
		case ast.Stmt:
			switch x.(type) {
			case *ast.BlockStmt:
				return true
			}
			v[i] = x
			stmts[x] = i
			i++
			//TODO skip over any statements w/i inner func... as our graph does
		}
		return true
	})
	v[END] = cfg.Exit
	v[START] = cfg.Entry
	stmts[cfg.Entry] = START
	stmts[cfg.Exit] = END
	if len(v) != len(cfg.Blocks()) {
		t.Logf("expected %d vertices, got %d --construction error", len(v), len(cfg.Blocks()))
	}

	return &CFGWrapper{
		cfg:      cfg,
		prog:     prog,
		exp:      v,
		stmts:    stmts,
		objs:     objs,
		objNames: objNames,
		fset:     prog.Fset,
		f:        f,
	}
}

func (c *CFGWrapper) expIntsToStmts(args []int) map[ast.Stmt]struct{} {
	stmts := make(map[ast.Stmt]struct{})
	for _, a := range args {
		stmts[c.exp[a]] = struct{}{}
	}
	return stmts
}

// give generics
func expectFromMaps(actual, exp map[ast.Stmt]struct{}) (dnf, found map[ast.Stmt]struct{}) {
	for stmt, _ := range exp {
		if _, ok := actual[stmt]; ok {
			delete(exp, stmt)
			delete(actual, stmt)
		}
	}
	return actual, exp
}

func (c *CFGWrapper) expectLive(t *testing.T, s int, exp ...string) {
	if _, ok := c.stmts[c.exp[s]]; !ok {
		t.Error("did not find parent", s)
		return
	}

	// get live for stmt s as slice, put in map
	actualLive := make(map[*types.Var]struct{})

	_, out := LiveVars(c.cfg, c.prog.Created[0])
	outs := out[c.exp[s]]
	for o, _ := range outs {
		actualLive[o] = struct{}{}
	}

	expLive := make(map[*types.Var]struct{})
	for _, e := range exp {
		expLive[c.objs[e]] = struct{}{}
	}

	for e, _ := range expLive {
		if _, ok := actualLive[e]; ok {
			delete(expLive, e)
			delete(actualLive, e)
		}
	}

	for obj, _ := range expLive {
		t.Error("did not find", c.objNames[obj], "as a live variable for", s)
	}

	for obj, _ := range actualLive {
		t.Error("found", c.objNames[obj], "as a live variable for", s)
	}
}

func (c *CFGWrapper) expectReaching(t *testing.T, s int, exp ...int) {
	if _, ok := c.stmts[c.exp[s]]; !ok {
		t.Error("did not find parent", s)
		return
	}

	// get reaching for stmt s as slice, put in map
	actualReach := make(map[ast.Stmt]struct{})
	// TODO(reed): test outs
	in, _ := ReachingDefs(c.cfg, c.prog.Created[0])
	ins := in[c.exp[s]]
	for i, _ := range ins {
		actualReach[i] = struct{}{}
	}

	expReach := c.expIntsToStmts(exp)
	dnf, found := expectFromMaps(actualReach, expReach)

	for stmt, _ := range found {
		t.Error("did not find", c.stmts[stmt], "in reaching for", s)
	}

	for stmt, _ := range dnf {
		t.Error("found", c.stmts[stmt], "as a reaching for", s)
	}
}

func (c *CFGWrapper) expectUses(t *testing.T, start int, end int, exp ...string) {
	if _, ok := c.stmts[c.exp[start]]; !ok {
		t.Error("did not find start", start)
		return
	}
	if _, ok := c.stmts[c.exp[end]]; !ok {
		t.Error("did not find end", end)
		return
	}

	var stmts []ast.Stmt
	for i := start; i <= end; i++ { // include end
		stmts = append(stmts, c.exp[i])
	}

	_, uses := ReferencedVars(stmts, c.prog.Created[0])

	actualUse := make(map[*types.Var]struct{})
	for u, _ := range uses {
		actualUse[u] = struct{}{}
	}

	expUse := make(map[*types.Var]struct{})
	for _, e := range exp {
		expUse[c.objs[e]] = struct{}{}
	}

	for d, _ := range expUse {
		if _, ok := actualUse[d]; ok {
			delete(expUse, d)
			delete(actualUse, d)
		}
	}

	for u, _ := range expUse {
		t.Error("Did not find", u.Name(), "in definitions")
	}
	for u, _ := range actualUse {
		t.Error("Found", u.Name(), "in definitions")
	}
}

func (c *CFGWrapper) expectDefs(t *testing.T, start int, end int, exp ...string) {
	if _, ok := c.stmts[c.exp[start]]; !ok {
		t.Error("did not find start", start)
		return
	}
	if _, ok := c.stmts[c.exp[end]]; !ok {
		t.Error("did not find end", end)
		return
	}

	var stmts []ast.Stmt
	for i := start; i <= end; i++ {
		stmts = append(stmts, c.exp[i])
	}

	defs, _ := ReferencedVars(stmts, c.prog.Created[0])

	actualDef := make(map[*types.Var]struct{})
	for d, _ := range defs {
		actualDef[d] = struct{}{}
	}

	expDef := make(map[*types.Var]struct{})
	for _, e := range exp {
		expDef[c.objs[e]] = struct{}{}
	}

	for d, _ := range expDef {
		if _, ok := actualDef[d]; ok {
			delete(expDef, d)
			delete(actualDef, d)
		}
	}

	for d, _ := range expDef {
		t.Error("Did not find", d.Name(), "in uses")
	}
	for f, _ := range actualDef {
		t.Error("Found", f.Name(), "in uses")
	}

}

//prints given AST
func (c *CFGWrapper) printAST() {
	ast.Print(c.fset, c.f)
}
