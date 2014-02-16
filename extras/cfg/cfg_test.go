//something something

package cfg

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestDoubleForBreak(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START 0
    for { //1
      for { //2
        break //3
      }
    }
    print("this") //4
    //END 5
  }`)
	//            t, stmt, ...successors
	c.expectSuccs(t, 0, 1)
	c.expectSuccs(t, 1, 2, 4)
	c.expectSuccs(t, 2, 3, 1)
	c.expectSuccs(t, 3, 1)

	c.expectPreds(t, 3, 2)
	c.expectPreds(t, 4, 1)
	c.expectPreds(t, 5, 4)
}

func TestFor(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START 0
    for i := 0; i < c; i++ { //2, 1, 3
      println(i) //4
    }
    println(c) //5
    //END 6
  }`)

	c.expectSuccs(t, 0, 1)
	c.expectSuccs(t, 2, 1)
	c.expectSuccs(t, 1, 4, 5)
	c.expectSuccs(t, 4, 3)
}

func TestIfElse(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START 0
    if c := 1; c > 0 { //1, 2
      print("there") //3
    } else {
      print("nowhere") //4
    }
    //END 5
  }`)

	c.expectSuccs(t, 0, 1)
	c.expectSuccs(t, 1, 2)
	c.expectSuccs(t, 2, 3, 4)

	c.expectPreds(t, 4, 2)
	c.expectPreds(t, 5, 4, 3)
	//TODO
}

func TestIfNoElse(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START 0
    if c > 0 { //1
      println("here") //2
    }
    print("there") //3
    //END //4
  }
  `)

	c.expectSuccs(t, 0, 1)
	c.expectSuccs(t, 1, 2, 3)

	c.expectPreds(t, 3, 1, 2)
	c.expectPreds(t, 4, 3)
}

func TestIfElseIf(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START 0
    if c > 0 { //1
      println("here") //2
    } else if c == 0 { //3
      println("there") //4
    } else {
      println("everywhere") //5
    }
    //END 6
  }`)

	c.expectSuccs(t, 0, 1)
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
    //START 0
    print("this") //1
    defer print("one") //2
    if 1 != 0 { //3
      defer print("two") //4
      return //5
    }
    print("that") //6
    defer print("three") //7
    return //8
    //END 9
  }
  `)
	c.expectSuccs(t, 0, 1)
	c.expectSuccs(t, 2, 9)
	c.expectSuccs(t, 5, 4)

	c.expectPreds(t, 7, 8)
	c.expectPreds(t, 9, 2)
	c.expectPreds(t, 2, 4)
	c.expectPreds(t, 5, 3)
	//TODO
}

//TODO little heavy, unit test better
func TestRange(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo() { 
    //START 0
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
    print("done") //9
    //END 10
  }
  `)
	c.expectSuccs(t, 0, 1)
	//TODO
}

func TestTypeSwitchDefault(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(s ast.Stmt) {
    //START 0
    switch s.(type) { //1, 2
    case *ast.AssignStmt: //3
      print("assign") //4
    case *ast.ForStmt: //5
      print("for") //6
    default: //7
      print("default") //8
    }
    print("done") //9
    //END 10
  }
  `)
	c.expectSuccs(t, 2, 3, 5, 7)
	//TODO
}

func TestSwitch(t *testing.T) {
	c := getWrapper(t, `
  package main
  
  func foo(c int) {
    //START 0
    print("hi") //1
    switch c+=1; c { //2, 3
    case 1: //4
      print("one") //5
      fallthrough //6
    case 2: //7
      break //8
      print("two") //9
    case 3: //10
    default: //11
      print("done") //12
    }
    print("bye") //13
    //END 14
  }
  `)
	c.expectSuccs(t, 0, 1)
	c.expectSuccs(t, 1, 2)
	c.expectSuccs(t, 2, 3)
	c.expectSuccs(t, 3, 4, 7, 10, 11)
	//TODO finish

	//preds meow...
	c.expectPreds(t, 13, 12, 10, 9, 8)
	//TODO finish
}

func TestLabeledFallthrough(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START 0
    switch c { //1
    case 1: //2
      print("one") //3
      goto lbl //4
    case 2: //5
      print("two") //6
    lbl: //7
      fallthrough //8
    default: //9
      print("number") //10
    }
    //END 11
  }`)

	c.expectSuccs(t, 0, 1)
	c.expectSuccs(t, 1, 2, 5, 9)
	c.expectSuccs(t, 4, 8)
	c.expectSuccs(t, 7, 8)
	c.expectSuccs(t, 8, 9)

	c.expectPreds(t, 11, 10)
}

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
	c.expectSuccs(t, 0, 1)
	//TODO ultimate stress test
}

//lo and behold how it's done -- caution: disgust may ensue
type CFGWrapper struct {
	cfg *CFG
	exp map[int]ast.Stmt
}

//uses first function in given string to produce CFG
func getWrapper(t *testing.T, str string) *CFGWrapper {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", str, 0)
	if err != nil {
		t.Error(err.Error())
		t.FailNow()
		return nil
	}
	cfg := FuncCFG(f.Decls[0].(*ast.FuncDecl)) //yes, so all test cases take first function
	//cfg.cfgtodot() //weird placement -- useful
	v, i := make(map[int]ast.Stmt), 1
	v[0] = cfg.start
	ast.Inspect(f.Decls[0].(*ast.FuncDecl), func(n ast.Node) bool {
		switch x := n.(type) {
		case ast.Stmt:
			switch x.(type) {
			case *ast.BlockStmt:
				return true
			}
			v[i] = x
			i++
		}
		return true
	})
	v[i] = cfg.end
	if len(v) != len(cfg.vMap) {
		t.Errorf("Expected %d vertices, got %d --construction error", len(v), len(cfg.vMap))
		//t.FailNow()
	}
	return &CFGWrapper{cfg, v}
}

func (c *CFGWrapper) expectSuccs(t *testing.T, s int, expSuccs ...int) {
	if _, ok := c.cfg.vMap[c.exp[s]]; !ok {
		t.Error("Did not find parent", s)
	}
	//TODO O(n^2)

	//get successors for stmt s as slice, put in map
	actualSuccs := make(map[ast.Stmt]bool)
	for _, v := range c.cfg.Succs(c.exp[s]) {
		actualSuccs[v] = true
	}

	for _, a := range expSuccs {
		if _, ok := actualSuccs[c.exp[a]]; !ok {
			t.Error("Did not find", a, "in successors for", s)
		} else {
			delete(actualSuccs, c.exp[a])
		}
	}

	//this asserts that the dingus writing the tests is in fact a dingus
	//TODO omit for ambiguities (later)?
	//also am I dumb or just plain stupid with this runtime?
	if len(actualSuccs) > 0 {
		for p, _ := range actualSuccs {
			for k, v := range c.exp {
				if p == v { //eventually it will...
					t.Error("Found", k, "as successor for", s)
				}
			}
		}
	}
}

func (c *CFGWrapper) expectPreds(t *testing.T, s int, expPreds ...int) {
	if _, ok := c.cfg.vMap[c.exp[s]]; !ok {
		t.Error("Did not find parent", s)
	}
	//TODO O(n^2) -- mostly small test cases, but potentially bad

	//get predecessors for stmt s as slice, put in map
	actualPreds := make(map[ast.Stmt]bool)
	for _, v := range c.cfg.Preds(c.exp[s]) {
		actualPreds[v] = true
	}

	for _, a := range expPreds {
		if _, ok := actualPreds[c.exp[a]]; !ok {
			t.Error("Did not find", a, "in predecessors for", s)
		} else {
			delete(actualPreds, c.exp[a])
		}
	}

	//re: me being an idiot
	if len(actualPreds) > 0 {
		for p, _ := range actualPreds {
			for k, v := range c.exp {
				if p == v { //eventually it will...
					t.Error("Found", k, "as a predecessor for", s)
				}
			}
		}
	}
}

//output a graph.dot file... for now
//most likely for testing only
//func (c *CFG) cfgtodot() {
//f, err := os.Create("graph.dot")
//if err != nil {
//panic(err)
//}
//fmt.Fprintf(f, `digraph mgraph {
//mode="heir";
//splines="ortho";

//`)
//for _, v := range c.vMap {
//for _, a := range v.succs {
//fmt.Fprintf(f, "\t\"%s\" -> \"%s\"\n", c.printVertex(v), c.printVertex(c.getVertex(a)))
//}
//}
//fmt.Fprintf(f, "}\n")
//}

//func (c *CFG) printVertex(v *vertex) string {
//switch v.stmt {
//case c.start:
//return fmt.Sprintf("%s %p", "START", v.stmt)
//case c.end:
//return fmt.Sprintf("%s %p", "END", v.stmt)
//}
//return printStmt(v.stmt)
//}

//func printStmt(s ast.Stmt) string {
//p := func(str string) string {
//return fmt.Sprintf("%s %p", str, s)
//}
//switch s.(type) {
//case *ast.CaseClause: //DONE
//return p("CASE")
//case *ast.CommClause: //DONE
//return p("COMM")
//case *ast.ForStmt: //DONE
//return p("FOR")
//case *ast.IfStmt: //DONE
//return p("IF")
//case *ast.AssignStmt: //DONE
//return p("ASSIGN")
//case *ast.BadStmt: //DONE
//return p("BAD")
//case *ast.BranchStmt: //DONE
//return p("BRANCH")
//case *ast.BlockStmt: //TODO where? use as entry?
//return p("BLOCK")
//case *ast.DeclStmt: //DONE
//return p("DECL")
//case *ast.DeferStmt: //TODO conditionals... done?
//return p("DEFER")
//case *ast.EmptyStmt: //DONE
//return p("EMPTY")
//case *ast.ExprStmt: //DONE
//return p("EXPR")
//case *ast.GoStmt: //DONE
//return p("GO")
//case *ast.IncDecStmt: //DONE
//return p("INCDEC")
//case *ast.LabeledStmt: //DONE
//return p("LABELED")
//case *ast.RangeStmt: //DONE
//return p("RANGE")
//case *ast.ReturnStmt: //DONE
//return p("RETURN")
//case *ast.SelectStmt: //DONE
//return p("SELECT")
//case *ast.SendStmt: //DONE
//return p("SEND")
//case *ast.SwitchStmt: //DONE
//return p("SWITCH")
//case *ast.TypeSwitchStmt: //DONE
//return p("TYPESWITCH")
//}
//return ""
//}

////leave this around for printing and things...
//func TestCFGPrint(t *testing.T) {
////TODO ast.Inspect and number nodes
////expectSuccs(#, #...)

////fset := token.NewFileSet() // positions are relative to fset
////f, err := parser.ParseFile(fset, "", `
////package main

////func foo(s ast.Stmt) {
//////START 0
////switch s.(type) { //1, 2
////case *ast.AssignStmt: //3
////print("assign") //4
////case *ast.ForStmt: //5
////print("for") //6
////default: //7
////print("default") //8
////}
////print("done") //9
//////END 10
////}
////`, 0)
////if err != nil {
////fmt.Println(err)
////}

//////// Print the AST.
////ast.Print(fset, f)
////cfg := MakeCFG(f.Decls[0].(*ast.FuncDecl))
////cfg.cfgtodot() //side effects: .dot file
//}
