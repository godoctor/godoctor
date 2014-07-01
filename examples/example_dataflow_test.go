package examples

import (
	"fmt"
	"go/ast"

	"code.google.com/p/go.tools/go/loader"

	"golang-refactoring.org/go-doctor/analysis/cfg"
	"golang-refactoring.org/go-doctor/analysis/dataflow"
)

func ExampleReachingDefs() {
	src := `
    package main

    import "fmt"

    func main() {
      a := 1
      b := 2
      c := 3
      a := b
      a, b := b, a
      c := a + b
    }
  `

	// use own loader config, this is just necessary
	var config loader.Config
	f, err := config.ParseFile("testing", src)
	if err != nil {
		return // probably don't proceed
	}
	pkg := loader.CreatePkg{"testing", []*ast.File{f}}
	config.CreatePkgs = []loader.CreatePkg{pkg}
	prog, err := config.Load()
	if err != nil {
		return
	}

	funcOne := f.Decls[1].(*ast.FuncDecl)
	c := cfg.FromFunc(funcOne)
	in, out := dataflow.ReachingDefs(c, prog.Created[0])

	ast.Inspect(f, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case ast.Stmt:
			ins, _ := in[stmt], out[stmt]
			fmt.Println(len(ins))
			// do as you please
		}
		return true
	})
}

func ExampleLiveVars() {
	src := `
    package main

    import "fmt"

    func main() {
      a := 1
      b := 2
      c := 3
      a := b
      a, b := b, a
      c := a + b
    }
  `

	// use own loader config, this is just necessary
	var config loader.Config
	f, err := config.ParseFile("testing", src)
	if err != nil {
		return // probably don't proceed
	}
	pkg := loader.CreatePkg{"testing", []*ast.File{f}}
	config.CreatePkgs = []loader.CreatePkg{pkg}
	prog, err := config.Load()
	if err != nil {
		return
	}

	funcOne := f.Decls[1].(*ast.FuncDecl)
	cfg := cfg.FromFunc(funcOne)
	in, out := dataflow.LiveVars(cfg, prog.Created[0])

	ast.Inspect(f, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case ast.Stmt:
			_, _ = in[stmt], out[stmt]
			// do as you please
		}
		return true
	})
}
