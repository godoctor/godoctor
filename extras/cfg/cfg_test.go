//something something

package cfg

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"

	"code.google.com/p/go.tools/astutil"
)

const (
	START = 0
	END   = 100000000 //if there's this many statements, may god have mercy on your soul
)

func TestExprStuff(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int, nums []int) {
    //START
    a := c      //1
    b := a      //2
    b = a + 1   //3
    c, a = a, c //4
    b = a       //5
    for a < c { //6
      a += c    //7
    }
    a, c = c, a //8
    c = b       //9
    return a    //10
    //END
  }`)

	c.expectReaching(t, START)
	c.expectReaching(t, 2, 1)
	c.expectReaching(t, 4, 3, 1)
	c.expectReaching(t, 6, 7, 5, 4)
	c.expectReaching(t, 7, 7, 5, 4)
	c.expectReaching(t, 8, 7, 5, 4)
	c.expectReaching(t, 9, 8, 5)

	//TODO not sure if these are right
	c.expectLive(t, START, "c")
	c.expectLive(t, 1, "a", "c")
	c.expectLive(t, 2, "a", "c")
	c.expectLive(t, 3, "a", "c")
	c.expectLive(t, 4, "a", "c")
	c.expectLive(t, 5, "a", "b", "c")
	c.expectLive(t, 8, "a", "b")
	c.expectLive(t, 9, "a")
	c.expectLive(t, END)

	//c.printAST()
}

func TestExractFuncExample(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo() {
    a := 3      //1
    b := a - 1  //2
    c := 1      //3

    // BEGIN EXTRACT
    for a < b { //4
      a += b    //5
    }
    x := a + b  //6
    // END EXTRACT
    
    z := x + c  //7
  }`)

	// only analyzes LIVE[OUT] for now...
	// but of criticial importance, worst case for EXTRACT FUNC:
	c.expectLive(t, 3, "a", "b", "c")
	c.expectLive(t, 6, "x", "c")

	// So in extracted function, "c" never gets used so we don't need to
	// pass it as a parameter nor return it since the value never changes
	// within our extracted function.
	// Yet, it's still live for the duration of the extracted function.
	// TODO Clever ideas
	//
	//    current ideas:
	//      B = BEGIN EXTRACT
	//      E = END EXTRACT
	//      PARAMS[EXTRACTED] = LIVE[IN][B] ∩ USE[EXTRACTED]
	//      RETURN[EXTRACTED] = LIVE[OUT][E] ∩ ? - ?
	//
	//      or generate LIVE[IN][B] and LIVE[OUT][E] for extracted function
	//      only in the context of the extracted function
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

func TestDoubleForBreak(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    for { //1
      for { //2
        break //3
      }
    }
    print("this") //4
    //END
  }`)

	//            t, stmt, ...successors
	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2, 4)
	c.expectSuccs(t, 2, 3, 1)
	c.expectSuccs(t, 3, 1)

	c.expectPreds(t, 3, 2)
	c.expectPreds(t, 4, 1)
	c.expectPreds(t, END, 4)
}

func TestFor(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    for i := 0; i < c; i++ { //2, 1, 3
      println(i) //4
    }
    println(c) //5
    //END
  }`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 2, 1)
	c.expectSuccs(t, 1, 4, 5)
	c.expectSuccs(t, 4, 3)

	c.expectPreds(t, END, 5)
}

func TestForContinue(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    for i := 0; i < c; i++ { //2, 1, 3
      println(i) //4
      if i > 1 { //5
        continue //6
      } else {
        break    //7
      }
    }
    println(c) //8
    //END
  }`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 2, 1)
	c.expectSuccs(t, 1, 4, 8)
	c.expectSuccs(t, 6, 3)
	c.expectSuccs(t, 7, 8)

	c.expectPreds(t, END, 8)
}

func TestIfElse(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    if c := 1; c > 0 { //1, 2
      print("there") //3
    } else {
      print("nowhere") //4
    }
    //END
  }`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2)
	c.expectSuccs(t, 2, 3, 4)

	c.expectPreds(t, 4, 2)
	c.expectPreds(t, END, 4, 3)
	//TODO
}

func TestIfNoElse(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    if c > 0 && true { //1
      println("here") //2
    }
    print("there") //3
    //END
  }
  `)
	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2, 3)

	c.expectPreds(t, 3, 1, 2)
	c.expectPreds(t, END, 3)
}

func TestIfElseIf(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    if c > 0 { //1
      println("here") //2
    } else if c == 0 { //3
      println("there") //4
    } else {
      println("everywhere") //5
    }
    print("almost end") //6
    //END
  }`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2, 3)
	c.expectSuccs(t, 2, 6)
	c.expectSuccs(t, 3, 4, 5)
	c.expectSuccs(t, 4, 6)
	c.expectSuccs(t, 5, 6)

	c.expectPreds(t, 6, 5, 4, 2)
}

func TestDefer(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo() {
    //START
    print("this") //1
    defer print("one") //2
    if 1 != 0 { //3
      defer print("two") //4
      return //5
    }
    print("that") //6
    defer print("three") //7
    return //8
    //END
  }
  `)
	c.expectSuccs(t, 3, 5, 6)
	c.expectSuccs(t, 5, 4)

	c.expectPreds(t, 7, 8)
	c.expectPreds(t, 4, 7, 5)
	c.expectPreds(t, 2, 4)
	c.expectPreds(t, 5, 3)
	//TODO
}

//TODO little heavy, unit test better
func TestRange(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo() { 
    //START
    c := []int{1, 2, 3} //1
  lbl: //2
    for i, v := range c { //3
      for j, k := range c { //4
        if i == j { //5
          break //6
        }
        print(i*i) //7
        break lbl //8
      }
    }
    //END
  }
  `)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 2, 3)
	c.expectSuccs(t, 3, 4, END)
	c.expectSuccs(t, 4, 5, 3)
	c.expectSuccs(t, 6, 3)
	c.expectSuccs(t, 8, END)
	//TODO why does preds work for 8, 3 but not succs?

	c.expectPreds(t, END, 8, 3)
}

func TestTypeSwitchDefault(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(s ast.Stmt) {
    //START
    switch s.(type) { //1, 2
    case *ast.AssignStmt: //3
      print("assign") //4
    case *ast.ForStmt: //5
      print("for") //6
    default: //7
      print("default") //8
    }
    //END
  }
  `)
	c.expectSuccs(t, 2, 3, 5, 7)

	c.expectPreds(t, END, 8, 6, 4)
	//TODO
}

//func TestTypeSwitchNoDefault(t *testing.T) {
//c := getWrapper(t, `
//package main

//func foo(s ast.Stmt) {
////START
//switch x := 1; s := s.(type) { //1, 2
//case *ast.AssignStmt: //3
//print("assign") //4
//case *ast.ForStmt: //5
//print("for") //6
//default: //7
//print("default") //8
//}
////END
//}
//`)
//c.expectSuccs(t, 2, 3, 5, 7)

//c.expectPreds(t, END, 8, 6, 4)
////TODO
//}

func TestSwitch(t *testing.T) {
	c := getWrapper(t, `
  package main
  
  func foo(c int) {
    //START
    print("hi") //1
    switch c+=1; c { //2, 3
    case 1: //4
      print("one") //5
      fallthrough //6
    case 2: //7
      break //8
      print("two") //9
    case 3: //10
    case 4: //11
      if i > 3 { //12
        print("> 3") //13
      } else { 
        print("< 3") //14
      }
    default: //15
      print("done") //16
    }
    //END
  }
  `)
	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2)
	c.expectSuccs(t, 2, 3)
	c.expectSuccs(t, 3, 4, 7, 10, 11, 15)
	//TODO finish

	//preds meow...
	c.expectPreds(t, END, 16, 14, 13, 10, 9, 8)
	//TODO finish
}

func TestLabeledFallthrough(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    switch c { //1
    case 1: //2
      print("one") //3
      goto lbl //4
    case 2: //5
      print("two") //6
    lbl: //7
      mlbl: //8
        fallthrough //9
    default: //10
      print("number") //11
    }
    //END
  }`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2, 5, 10)
	c.expectSuccs(t, 4, 7)
	c.expectSuccs(t, 7, 8)
	c.expectSuccs(t, 8, 9)
	c.expectSuccs(t, 9, 10)
	c.expectSuccs(t, 10, 11)

	c.expectPreds(t, END, 11)
}

// TODO modify ast.Inspect for go statements
// TODO also, does a go statement have control ever?
//func TestClosure(t *testing.T) {
//c := getWrapper(t, `
//package main

//func foo(c int) {
////START
//if c > 0 { //1
//go func(i int) { //2
//println(i)
//}(c)
//}
//println(c) //3
////END
//}`)

//c.printAST()
//c.printDOT()

//c.expectSuccs(t, START, 1)
//c.expectSuccs(t, 1, 2, 3)
//}

func TestDietyExistence(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    b := 7
  hello:
    for c < b {
      for {
        if c&2 == 2 {
          continue hello
          println(even)
        } else if c&1 == 1 {
          defer println(sup)
          println(odd)
          break
        } else {
          println("something wrong")
          goto ending
        }
        println("something")
      }
      println("poo")
    }
    println("hello")
    ending:
  }
  `)
	c.expectSuccs(t, START, 1)
	//TODO ultimate stress test
}

// lo and behold how it's done -- caution: disgust may ensue
type CFGWrapper struct {
	cfg   *CFG
	exp   map[int]ast.Stmt
	stmts map[ast.Stmt]int
	objs  map[string]*ast.Object
	fset  *token.FileSet
	f     *ast.File
}

// uses first function in given string to produce CFG
// w/ some other convenient fields for printing in test
// cases when need be...
func getWrapper(t *testing.T, str string) *CFGWrapper {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", str, 0)
	if err != nil {
		t.Error(err.Error())
		t.FailNow()
		return nil
	}
	cfg := FuncCFG(f.Decls[0].(*ast.FuncDecl)) //yes, so all test cases take first function
	v := make(map[int]ast.Stmt)
	stmts := make(map[ast.Stmt]int)
	objs := make(map[string]*ast.Object)
	i := 1
	ast.Inspect(f.Decls[0].(*ast.FuncDecl), func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			objs[x.Name] = x.Obj
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
	v[END] = cfg.end
	v[START] = cfg.start
	if len(v) != len(cfg.bMap) {
		t.Errorf("expected %d vertices, got %d --construction error", len(v), len(cfg.bMap))
		//t.FailNow()
	}
	return &CFGWrapper{cfg, v, stmts, objs, fset, f}
}

func (c *CFGWrapper) expIntsToStmts(args []int) map[ast.Stmt]bool {
	stmts := make(map[ast.Stmt]bool)
	for _, a := range args {
		stmts[c.exp[a]] = true
	}
	return stmts
}

// give generics
func expectFromMaps(actual map[ast.Stmt]bool, exp map[ast.Stmt]bool) (dnf []ast.Stmt, found []ast.Stmt) {
	for stmt, _ := range exp {
		if _, ok := actual[stmt]; !ok {
			dnf = append(dnf, stmt)
		} else {
			delete(actual, stmt)
		}
	}

	for stmt, _ := range actual {
		found = append(found, stmt)
	}

	return
}

func (c *CFGWrapper) expectLive(t *testing.T, s int, exp ...string) {
	if _, ok := c.cfg.bMap[c.exp[s]]; !ok {
		t.Error("did not find parent", s)
		return
	}

	// for names
	objMap := make(map[*ast.Object]string)
	for name, obj := range c.objs {
		objMap[obj] = name
	}

	// get live for stmt s as slice, put in map
	actualLive := make(map[*ast.Object]bool)

	_, outs := c.cfg.Live(c.exp[s])
	for _, o := range outs {
		actualLive[o] = true
	}

	// TODO(reed): appears to be duplicates? is this the testing's fault?
	//for a, _ := range actualLive {
	//fmt.Println(objMap[a])
	//}

	expLive := make(map[*ast.Object]bool)
	for _, e := range exp {
		expLive[c.objs[e]] = true
	}

	var dnf, found []*ast.Object
	for e, _ := range expLive {
		if _, ok := actualLive[e]; !ok {
			dnf = append(dnf, e)
		} else {
			delete(actualLive, e)
		}
	}

	for e, _ := range actualLive {
		found = append(found, e)
	}

	for _, obj := range dnf {
		t.Error("did not find", objMap[obj], "as a live variable for", s)
	}

	for _, obj := range found {
		t.Error("found", objMap[obj], "as a live variable for", s)
	}
}

func (c *CFGWrapper) expectReaching(t *testing.T, s int, exp ...int) {
	if _, ok := c.cfg.bMap[c.exp[s]]; !ok {
		t.Error("did not find parent", s)
		return
	}

	// get reaching for stmt s as slice, put in map
	actualReach := make(map[ast.Stmt]bool)
	// TODO(reed): test outs
	ins, _ := c.cfg.Reaching(c.exp[s])
	for _, i := range ins {
		actualReach[i] = true
	}

	expReach := c.expIntsToStmts(exp)
	dnf, found := expectFromMaps(actualReach, expReach)

	for _, stmt := range dnf {
		t.Error("did not find", c.stmts[stmt], "in reaching for", s)
	}

	for _, stmt := range found {
		t.Error("found", c.stmts[stmt], "as a reaching for", s)
	}
}

func (c *CFGWrapper) expectSuccs(t *testing.T, s int, exp ...int) {
	if _, ok := c.cfg.bMap[c.exp[s]]; !ok {
		t.Error("did not find parent", s)
		return
	}

	//get successors for stmt s as slice, put in map
	actualSuccs := make(map[ast.Stmt]bool)
	for _, v := range c.cfg.Succs(c.exp[s]) {
		actualSuccs[v] = true
	}

	expSuccs := c.expIntsToStmts(exp)
	dnf, found := expectFromMaps(actualSuccs, expSuccs)

	for _, stmt := range dnf {
		t.Error("did not find", c.stmts[stmt], "in successors for", s)
	}

	for _, stmt := range found {
		t.Error("found", c.stmts[stmt], "as a successors for", s)
	}
}

func (c *CFGWrapper) expectPreds(t *testing.T, s int, exp ...int) {
	if _, ok := c.cfg.bMap[c.exp[s]]; !ok {
		t.Error("did not find parent", s)
	}

	//get predecessors for stmt s as slice, put in map
	actualPreds := make(map[ast.Stmt]bool)
	for _, v := range c.cfg.Preds(c.exp[s]) {
		actualPreds[v] = true
	}

	expPreds := c.expIntsToStmts(exp)
	dnf, found := expectFromMaps(actualPreds, expPreds)

	for _, stmt := range dnf {
		t.Error("did not find", c.stmts[stmt], "in predecessors for", s)
	}

	for _, stmt := range found {
		t.Error("found", c.stmts[stmt], "as a predecessor for", s)
	}
}

//prints given AST
func (c *CFGWrapper) printAST() {
	ast.Print(c.fset, c.f)
}

//output a graph.dot file... for now
//most likely for testing only
func (c *CFGWrapper) printDOT() {
	f, err := os.Create("graph.dot")
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(f, `digraph mgraph {
mode="heir";
splines="ortho";

`)
	for _, v := range c.cfg.bMap {
		for _, a := range v.succs {
			fmt.Fprintf(f, "\t\"%s\" -> \"%s\"\n", c.printVertex(v), c.printVertex(c.cfg.bMap[a]))
		}
	}
	fmt.Fprintf(f, "}\n")
}

func (c *CFGWrapper) printVertex(v *block) string {
	switch v.stmt {
	case c.cfg.start:
		return fmt.Sprintf("%s %p", "START", v.stmt)
	case c.cfg.end:
		return fmt.Sprintf("%s %p", "END", v.stmt)
	}
	return fmt.Sprintf("%s %p", astutil.NodeDescription(v.stmt), v.stmt)
}
